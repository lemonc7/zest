package zest

import (
	"encoding"
	"encoding/json"
	"encoding/xml"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Validator interface {
	Validate() error
}

func (c *Context) Bind(dst Validator) error {
	if err := bindPathValues(c.Request, dst); err != nil {
		return err
	}

	method := c.Request.Method
	if method == http.MethodGet ||
		method == http.MethodDelete ||
		method == http.MethodHead {
		if err := bindQueryParams(c.Request, dst); err != nil {
			return err
		}
	}

	if err := bindBody(c.Request, dst); err != nil {
		return err
	}

	if err := dst.Validate(); err != nil {
		return NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	return nil
}

const defaultMemory = 32 << 20 // 32 MB
var (
	// NOT supported by bind as you can NOT check easily empty struct being actual file or not
	multipartFileHeaderType = reflect.TypeFor[multipart.FileHeader]()
	// supported by bind as you can check by nil value if file existed or not
	multipartFileHeaderPointerType      = reflect.TypeFor[*multipart.FileHeader]()
	multipartFileHeaderSliceType        = reflect.TypeFor[[]multipart.FileHeader]()
	multipartFileHeaderPointerSliceType = reflect.TypeFor[[]*multipart.FileHeader]()

	// 预编译路径参数正则表达式，匹配 {paramName} 格式
	pathParamRegex = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
)

// tag: param
func bindPathValues(req *http.Request, dst Validator) error {
	names := getPathParamNames(req.Pattern)
	params := map[string][]string{}
	for _, name := range names {
		value := req.PathValue(name)
		params[name] = []string{value}
	}

	if err := bindData(dst, params, "param", nil); err != nil {
		return NewHTTPError(http.StatusBadRequest).Wrap(err)
	}
	return nil
}

// tag: query
func bindQueryParams(req *http.Request, dst Validator) error {
	if err := bindData(dst, req.URL.Query(), "query", nil); err != nil {
		return NewHTTPError(http.StatusBadRequest).Wrap(err)
	}
	return nil
}

// tag: json
func bindBody(req *http.Request, dst Validator) (err error) {
	if req.ContentLength == 0 {
		return
	}
	base, _, _ := strings.Cut(req.Header.Get(HeaderContentType), ";")
	mediaType := strings.TrimSpace(base)

	switch mediaType {
	case MIMEApplicationJSON:
		if err = json.NewDecoder(req.Body).Decode(dst); err != nil {
			return NewHTTPError(http.StatusBadRequest).Wrap(err)
		}
	case MIMEApplicationXML, MIMETextXML:
		if err = xml.NewDecoder(req.Body).Decode(dst); err != nil {
			return NewHTTPError(http.StatusBadRequest).Wrap(err)
		}
	case MIMEApplicationForm:
		params, err := formParams(req)
		if err != nil {
			return NewHTTPError(http.StatusBadRequest).Wrap(err)
		}
		if err = bindData(dst, params, "form", nil); err != nil {
			return NewHTTPError(http.StatusBadRequest).Wrap(err)
		}
	case MIMEMultipartForm:
		if err = req.ParseMultipartForm(defaultMemory); err != nil {
			return NewHTTPError(http.StatusBadRequest).Wrap(err)
		}
		params := req.MultipartForm
		if err = bindData(dst, params.Value, "form", params.File); err != nil {
			return NewHTTPError(http.StatusBadRequest).Wrap(err)
		}
	default:
		return NewHTTPError(http.StatusUnsupportedMediaType)
	}
	return nil
}

func getPathParamNames(pattern string) []string {
	matches := pathParamRegex.FindAllStringSubmatch(pattern, -1)
	var params []string
	for _, match := range matches {
		params = append(params, match[1])
	}
	return params
}

// bindData will bind data ONLY fields in destination struct that have EXPLICIT tag
func bindData(
	dst any,
	data map[string][]string,
	tag string,
	dataFiles map[string][]*multipart.FileHeader,
) error {
	if dst == nil || (len(data) == 0 && len(dataFiles) == 0) {
		return nil
	}
	hasFiles := len(dataFiles) > 0
	typ := reflect.TypeOf(dst).Elem()
	val := reflect.ValueOf(dst).Elem()

	// Support binding to limited Map destinations:
	// - map[string][]string,
	// - map[string]string <-- (binds first value from data slice)
	// - map[string]interface{}
	// You are better off binding to struct but there are user who want this map feature. Source of data for these cases are:
	// params,query,header,form as these sources produce string values, most of the time slice of strings, actually.
	if typ.Kind() == reflect.Map && typ.Key().Kind() == reflect.String {
		k := typ.Elem().Kind()
		isElemInterface := k == reflect.Interface
		isElemString := k == reflect.String
		isElemSliceOfStrings := k == reflect.Slice && typ.Elem().Elem().Kind() == reflect.String
		if !(isElemSliceOfStrings || isElemString || isElemInterface) {
			return nil
		}
		if val.IsNil() {
			val.Set(reflect.MakeMap(typ))
		}
		for k, v := range data {
			if isElemString {
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v[0]))
			} else if isElemInterface {
				// To maintain backward compatibility, we always bind to the first string value
				// and not the slice of strings when dealing with map[string]interface{}{}
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v[0]))
			} else {
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
			}
		}
		return nil
	}

	// !struct
	if typ.Kind() != reflect.Struct {
		if tag == "param" || tag == "query" || tag == "header" {
			// incompatible type, data is probably to be found in the body
			return nil
		}
		return errors.New("binding element must be a struct")
	}

	for i := range typ.NumField() { // iterate over all destination fields
		typeField := typ.Field(i)
		structField := val.Field(i)
		if typeField.Anonymous {
			if structField.Kind() == reflect.Pointer {
				structField = structField.Elem()
			}
		}
		if !structField.CanSet() {
			continue
		}
		structFieldKind := structField.Kind()
		inputFieldName := typeField.Tag.Get(tag)
		if typeField.Anonymous && structFieldKind == reflect.Struct && inputFieldName != "" {
			// if anonymous struct with query/param/form tags, report an error
			return errors.New("query/param/form tags are not allowed with anonymous struct field")
		}

		if inputFieldName == "" {
			// If tag is nil, we inspect if the field is a not BindUnmarshaler struct and try to bind data into it (might contain fields with tags).
			// structs that implement BindUnmarshaler are bound only when they have explicit tag
			if _, ok := structField.Addr().Interface().(interface{ UnmarshalParam(param string) error }); !ok && structFieldKind == reflect.Struct {
				if err := bindData(structField.Addr().Interface(), data, tag, dataFiles); err != nil {
					return err
				}
			}
			// does not have explicit tag and is not an ordinary struct - so move to next field
			continue
		}

		if hasFiles {
			if ok, err := isFieldMultipartFile(structField.Type()); err != nil {
				return err
			} else if ok {
				if ok := setMultipartFileHeaderTypes(structField, inputFieldName, dataFiles); ok {
					continue
				}
			}
		}

		inputValue, exists := data[inputFieldName]
		if !exists {
			// Go json.Unmarshal supports case-insensitive binding.  However the
			// url params are bound case-sensitive which is inconsistent.  To
			// fix this we must check all of the map values in a
			// case-insensitive search.
			for k, v := range data {
				if strings.EqualFold(k, inputFieldName) {
					inputValue = v
					exists = true
					break
				}
			}
		}

		if !exists {
			continue
		}

		// NOTE: algorithm here is not particularly sophisticated. It probably does not work with absurd types like `**[]*int`
		// but it is smart enough to handle niche cases like `*int`,`*[]string`,`[]*int` .

		// try unmarshalling first, in case we're dealing with an alias to an array type
		if ok, err := unmarshalInputsToField(typeField.Type.Kind(), inputValue, structField); ok {
			if err != nil {
				return err
			}
			continue
		}

		if ok, err := unmarshalInputToField(typeField.Type.Kind(), inputValue[0], structField); ok {
			if err != nil {
				return err
			}
			continue
		}

		// we could be dealing with pointer to slice `*[]string` so dereference it. There are weird OpenAPI generators
		// that could create struct fields like that.
		if structFieldKind == reflect.Pointer {
			structFieldKind = structField.Elem().Kind()
			structField = structField.Elem()
		}

		if structFieldKind == reflect.Slice {
			sliceOf := structField.Type().Elem().Kind()
			numElems := len(inputValue)
			slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
			for j := range numElems {
				if err := setWithProperType(sliceOf, inputValue[j], slice.Index(j)); err != nil {
					return err
				}
			}
			structField.Set(slice)
			continue
		}

		if err := setWithProperType(structFieldKind, inputValue[0], structField); err != nil {
			return err
		}
	}
	return nil
}

func isFieldMultipartFile(field reflect.Type) (bool, error) {
	switch field {
	case multipartFileHeaderPointerType,
		multipartFileHeaderSliceType,
		multipartFileHeaderPointerSliceType:
		return true, nil
	case multipartFileHeaderType:
		return true, errors.New("binding to multipart.FileHeader struct is not supported, use pointer to struct")
	default:
		return false, nil
	}
}

func setMultipartFileHeaderTypes(
	structField reflect.Value,
	inputFieldName string,
	files map[string][]*multipart.FileHeader,
) bool {
	fileHeaders := files[inputFieldName]
	if len(fileHeaders) == 0 {
		return false
	}

	result := true
	switch structField.Type() {
	case multipartFileHeaderPointerSliceType:
		structField.Set(reflect.ValueOf(fileHeaders))
	case multipartFileHeaderSliceType:
		headers := make([]multipart.FileHeader, len(fileHeaders))
		for i, fileHeader := range fileHeaders {
			headers[i] = *fileHeader
		}
		structField.Set(reflect.ValueOf(headers))
	case multipartFileHeaderPointerType:
		structField.Set(reflect.ValueOf(fileHeaders[0]))
	default:
		result = false
	}
	return result
}

func unmarshalInputsToField(valueKind reflect.Kind, values []string, field reflect.Value) (bool, error) {
	if valueKind == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	fieldIValue := field.Addr().Interface()
	unmarshaler, ok := fieldIValue.(interface{ UnmarshalParams(params []string) error })
	if !ok {
		return false, nil
	}
	return true, unmarshaler.UnmarshalParams(values)
}

func unmarshalInputToField(valueKind reflect.Kind, val string, field reflect.Value) (bool, error) {
	if valueKind == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	fieldIValue := field.Addr().Interface()
	switch unmarshaler := fieldIValue.(type) {
	case interface{ UnmarshalParam(param string) error }:
		return true, unmarshaler.UnmarshalParam(val)
	case encoding.TextUnmarshaler:
		return true, unmarshaler.UnmarshalText([]byte(val))
	}

	return false, nil
}

func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value) error {
	// But also call it here, in case we're dealing with an array of BindUnmarshalers
	if ok, err := unmarshalInputToField(valueKind, val, structField); ok {
		return err
	}

	switch valueKind {
	case reflect.Pointer:
		return setWithProperType(structField.Elem().Kind(), val, structField.Elem())
	case reflect.Int:
		return setIntField(val, 0, structField)
	case reflect.Int8:
		return setIntField(val, 8, structField)
	case reflect.Int16:
		return setIntField(val, 16, structField)
	case reflect.Int32:
		return setIntField(val, 32, structField)
	case reflect.Int64:
		return setIntField(val, 64, structField)
	case reflect.Uint:
		return setUintField(val, 0, structField)
	case reflect.Uint8:
		return setUintField(val, 8, structField)
	case reflect.Uint16:
		return setUintField(val, 16, structField)
	case reflect.Uint32:
		return setUintField(val, 32, structField)
	case reflect.Uint64:
		return setUintField(val, 64, structField)
	case reflect.Bool:
		return setBoolField(val, structField)
	case reflect.Float32:
		return setFloatField(val, 32, structField)
	case reflect.Float64:
		return setFloatField(val, 64, structField)
	case reflect.String:
		structField.SetString(val)
	default:
		return errors.New("unknown type")
	}
	return nil
}

func setIntField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	intVal, err := strconv.ParseInt(value, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

func setUintField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	uintVal, err := strconv.ParseUint(value, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

func setBoolField(value string, field reflect.Value) error {
	if value == "" {
		value = "false"
	}
	boolVal, err := strconv.ParseBool(value)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

func setFloatField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0.0"
	}
	floatVal, err := strconv.ParseFloat(value, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}

func formParams(r *http.Request) (url.Values, error) {
	if strings.HasPrefix(r.Header.Get(HeaderContentType), MIMEMultipartForm) {
		if err := r.ParseMultipartForm(defaultMemory); err != nil {
			return nil, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	}
	return r.Form, nil
}
