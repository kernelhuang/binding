package binding

import (
	"mime/multipart"
	"reflect"
)

// Form is middleware to deserialize form-urlencoded data from the request.
// It gets data from the form-urlencoded body, if present, or from the
// query string. It uses the http.Request.ParseForm() method
// to perform deserialization, then reflection is used to map each field
// into the struct with the proper type. Structs with primitive slice types
// (bool, float, int, string) can support deserialization of repeated form
// keys, for example: key=val1&key=val2&key=val3
// An interface pointer can be added as a second argument in order
// to map the struct to a specific interface.
func (vd Binder) MapForm(obj interface{}) Errors {
	var errors Errors

	ensurePointer(obj)
	parseErr := vd.req.ParseForm()

	// Format validation of the request body or the URL would add considerable overhead,
	// and ParseForm does not complain when URL encoding is off.
	// Because an empty request body or url can also mean absence of all needed values,
	// it is not in all cases a bad request, so let's return 422.
	if parseErr != nil {
		errors.Add([]string{}, ERR_DESERIALIZATION, parseErr.Error())
	}
	mapForm(reflect.ValueOf(obj), vd.req.Form, nil, errors)
	errors = append(errors, vd.Validate(obj)...)
	return errors
}

// Takes values from the form data and puts them into a struct
func mapForm(formStruct reflect.Value, form map[string][]string,
	formfile map[string][]*multipart.FileHeader, errors Errors) {
	if formStruct.Kind() == reflect.Ptr {
		formStruct = formStruct.Elem()
	}

	typ := formStruct.Type()

	for i := 0; i < typ.NumField(); i++ {
		typeField := typ.Field(i)
		structField := formStruct.Field(i)

		if typeField.Type.Kind() == reflect.Ptr && typeField.Anonymous {
			structField.Set(reflect.New(typeField.Type.Elem()))
			mapForm(structField.Elem(), form, formfile, errors)
			if reflect.DeepEqual(structField.Elem().Interface(), reflect.Zero(structField.Elem().Type()).Interface()) {
				structField.Set(reflect.Zero(structField.Type()))
			}
		} else if typeField.Type.Kind() == reflect.Struct {
			mapForm(structField, form, formfile, errors)
		}

		inputFieldName := parseFormName(typeField.Name, typeField.Tag.Get("form"))
		if len(inputFieldName) > 0 {
			if !structField.CanSet() {
				continue
			}

			inputValue, exists := form[inputFieldName]
			if exists {
				numElems := len(inputValue)
				if structField.Kind() == reflect.Slice && numElems > 0 {
					sliceOf := structField.Type().Elem().Kind()
					slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
					for i := 0; i < numElems; i++ {
						setWithProperType(sliceOf, inputValue[i], slice.Index(i), inputFieldName, errors)
					}
					formStruct.Field(i).Set(slice)
				} else {
					setWithProperType(typeField.Type.Kind(), inputValue[0], structField, inputFieldName, errors)
				}
				continue
			}

			inputFile, exists := formfile[inputFieldName]
			if !exists {
				continue
			}
			fhType := reflect.TypeOf((*multipart.FileHeader)(nil))
			numElems := len(inputFile)
			if structField.Kind() == reflect.Slice && numElems > 0 && structField.Type().Elem() == fhType {
				slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
				for i := 0; i < numElems; i++ {
					slice.Index(i).Set(reflect.ValueOf(inputFile[i]))
				}
				structField.Set(slice)
			} else if structField.Type() == fhType {
				structField.Set(reflect.ValueOf(inputFile[0]))
			}
		}
	}
}

// Maximum amount of memory to use when parsing a multipart form.
// Set this to whatever value you prefer; default is 10 MB.
var MaxMemory = int64(1024 * 1024 * 10)

// MultipartForm works much like Form, except it can parse multipart forms
// and handle file uploads. Like the other deserialization middleware handlers,
// you can pass in an interface to make the interface available for injection
// into other handlers later.
func (vd Binder) MultipartForm(obj interface{}) Errors {
	var errors Errors
	ensurePointer(obj)

	// This if check is necessary due to https://github.com/martini-contrib/csrf/issues/6
	if vd.req.MultipartForm == nil {
		// Workaround for multipart forms returning nil instead of an error
		// when content is not multipart; see https://code.google.com/p/go/issues/detail?id=6334
		if multipartReader, err := vd.req.MultipartReader(); err != nil {
			errors.Add([]string{}, ERR_DESERIALIZATION, err.Error())
		} else {
			form, parseErr := multipartReader.ReadForm(MaxMemory)
			if parseErr != nil {
				errors.Add([]string{}, ERR_DESERIALIZATION, parseErr.Error())
			}
			vd.req.MultipartForm = form
		}
	}
	mapForm(reflect.ValueOf(obj), vd.req.MultipartForm.Value, vd.req.MultipartForm.File, errors)
	errors = append(errors, vd.Validate(obj)...)
	return errors
}
