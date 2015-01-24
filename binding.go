// Copyright 2014 martini-contrib/binding Authors
// Copyright 2014 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package binding is a middleware that provides request data binding and validation for Macaron.
package binding

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/lunny/tango"
)

// NOTE: last sync 1928ed2 on Aug 26, 2014.

const _VERSION = "0.0.4"

func Version() string {
	return _VERSION
}

type Options struct {
}

// Validate is middleware to enforce required fields. If the struct
// passed in implements Validator, then the user-defined Validate method
// is executed, and its errors are mapped to the context. This middleware
// performs no error handling: it merely detects errors and maps them.
func Bind(options ...Options) tango.HandlerFunc {
	return func(ctx *tango.Context) {
		if action := ctx.Action(); action != nil {
			if v, ok := action.(Bindinger); ok {
				v.SetBinder(ctx.Req())
			}
		}

		ctx.Next()
	}
}

type Bindinger interface {
	SetBinder(*http.Request)
}

type Binder struct {
	req *http.Request
}

func (vd *Binder) SetBinder(req *http.Request) {
	vd.req = req
}

func (vd Binder) Bind(obj interface{}) Errors {
	contentType := vd.req.Header.Get("Content-Type")
	if vd.req.Method == "POST" || vd.req.Method == "PUT" || len(contentType) > 0 {
		switch {
		case strings.Contains(contentType, "form-urlencoded"):
			return vd.MapForm(obj)
		case strings.Contains(contentType, "multipart/form-data"):
			return vd.MultipartForm(obj)
		case strings.Contains(contentType, "json"):
			return vd.Json(obj)
		default:
			var errors Errors
			if contentType == "" {
				errors.Add([]string{}, ERR_CONTENT_TYPE, "Empty Content-Type")
			} else {
				errors.Add([]string{}, ERR_CONTENT_TYPE, "Unsupported Content-Type")
			}
			return errors
		}
	} else {
		return vd.MapForm(obj)
	}
}

const (
	_JSON_CONTENT_TYPE          = "application/json; charset=utf-8"
	STATUS_UNPROCESSABLE_ENTITY = 422
)

// errorHandler simply counts the number of errors in the
// context and, if more than 0, writes a response with an
// error code and a JSON payload describing the errors.
// The response will have a JSON content-type.
// Middleware remaining on the stack will not even see the request
// if, by this point, there are any errors.
// This is a "default" handler, of sorts, and you are
// welcome to use your own instead. The Bind middleware
// invokes this automatically for convenience.
func errorHandler(errs Errors, rw http.ResponseWriter) {
	if len(errs) > 0 {
		rw.Header().Set("Content-Type", _JSON_CONTENT_TYPE)
		if errs.Has(ERR_DESERIALIZATION) {
			rw.WriteHeader(http.StatusBadRequest)
		} else if errs.Has(ERR_CONTENT_TYPE) {
			rw.WriteHeader(http.StatusUnsupportedMediaType)
		} else {
			rw.WriteHeader(STATUS_UNPROCESSABLE_ENTITY)
		}
		errOutput, _ := json.Marshal(errs)
		rw.Write(errOutput)
		return
	}
}

var (
	alphaDashPattern    = regexp.MustCompile("[^\\d\\w-_]")
	alphaDashDotPattern = regexp.MustCompile("[^\\d\\w-_\\.]")
	emailPattern        = regexp.MustCompile("[\\w!#$%&'*+/=?^_`{|}~-]+(?:\\.[\\w!#$%&'*+/=?^_`{|}~-]+)*@(?:[\\w](?:[\\w-]*[\\w])?\\.)+[a-zA-Z0-9](?:[\\w-]*[\\w])?")
	urlPattern          = regexp.MustCompile(`(http|https):\/\/[\w\-_]+(\.[\w\-_]+)+([\w\-\.,@?^=%&amp;:/~\+#]*[\w\-\@?^=%&amp;/~\+#])?`)
)

type (
	// Rule represents a validation rule.
	Rule struct {
		// IsMatch checks if rule matches.
		IsMatch func(string) bool
		// IsValid applies validation rule to condition.
		IsValid func(Errors, string, interface{}) bool
	}
	// RuleMapper represents a validation rule mapper,
	// it allwos users to add custom validation rules.
	RuleMapper []*Rule
)

var ruleMapper RuleMapper

// AddRule adds new validation rule.
func AddRule(r *Rule) {
	ruleMapper = append(ruleMapper, r)
}

func in(fieldValue interface{}, arr string) bool {
	val := fmt.Sprintf("%v", fieldValue)
	vals := strings.Split(arr, ",")
	isIn := false
	for _, v := range vals {
		if v == val {
			isIn = true
			break
		}
	}
	return isIn
}

func parseFormName(raw, actual string) string {
	if len(actual) > 0 {
		return actual
	}
	return nameMapper(raw)
}

// NameMapper represents a form/json tag name mapper.
type NameMapper func(string) string

var (
	nameMapper = func(field string) string {
		newstr := make([]rune, 0, 10)
		for i, chr := range field {
			if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
				if i > 0 {
					newstr = append(newstr, '_')
				}
				chr -= ('A' - 'a')
			}
			newstr = append(newstr, chr)
		}
		return string(newstr)
	}
)

// SetNameMapper sets name mapper.
func SetNameMapper(nm NameMapper) {
	nameMapper = nm
}

// This sets the value in a struct of an indeterminate type to the
// matching value from the request (via Form middleware) in the
// same type, so that not all deserialized values have to be strings.
// Supported types are string, int, float, and bool.
func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value, nameInTag string, errors Errors) {
	switch valueKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val == "" {
			val = "0"
		}
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			errors.Add([]string{nameInTag}, ERR_INTERGER_TYPE, "Value could not be parsed as integer")
		} else {
			structField.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if val == "" {
			val = "0"
		}
		uintVal, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			errors.Add([]string{nameInTag}, ERR_INTERGER_TYPE, "Value could not be parsed as unsigned integer")
		} else {
			structField.SetUint(uintVal)
		}
	case reflect.Bool:
		if val == "on" {
			structField.SetBool(true)
			return
		}

		if val == "" {
			val = "false"
		}
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			errors.Add([]string{nameInTag}, ERR_BOOLEAN_TYPE, "Value could not be parsed as boolean")
		} else if boolVal {
			structField.SetBool(true)
		}
	case reflect.Float32:
		if val == "" {
			val = "0.0"
		}
		floatVal, err := strconv.ParseFloat(val, 32)
		if err != nil {
			errors.Add([]string{nameInTag}, ERR_FLOAT_TYPE, "Value could not be parsed as 32-bit float")
		} else {
			structField.SetFloat(floatVal)
		}
	case reflect.Float64:
		if val == "" {
			val = "0.0"
		}
		floatVal, err := strconv.ParseFloat(val, 64)
		if err != nil {
			errors.Add([]string{nameInTag}, ERR_FLOAT_TYPE, "Value could not be parsed as 64-bit float")
		} else {
			structField.SetFloat(floatVal)
		}
	case reflect.String:
		structField.SetString(val)
	}
}

// Don't pass in pointers to bind to. Can lead to bugs.
func ensureNotPointer(obj interface{}) {
	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		panic("Pointers are not accepted as binding models")
	}
}

// Don't pass in pointers to bind to. Can lead to bugs.
func ensurePointer(obj interface{}) {
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		panic("Pointers are only accepted as binding models")
	}
}
