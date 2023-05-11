package binding

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/unknwon/com"
)

// Validator is the interface that handles some rudimentary
// request validation logic so your application doesn't have to.
type Validator interface {
	// Validate validates that the request is OK. It is recommended
	// that validation be limited to checking values for syntax and
	// semantics, enough to know that you can make sense of the request
	// in your application. For example, you might verify that a credit
	// card number matches a valid pattern, but you probably wouldn't
	// perform an actual credit card authorization here.
	Validate(*http.Request, Errors) Errors
}

func (vd Binder) Validate(obj interface{}) Errors {
	var errors Errors
	v := reflect.ValueOf(obj)
	k := v.Kind()
	if k == reflect.Interface || k == reflect.Ptr {
		v = v.Elem()
		k = v.Kind()
	}
	if k == reflect.Slice || k == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i).Interface()
			errors = validateStruct(errors, e)
			if validator, ok := e.(Validator); ok {
				errors = validator.Validate(vd.req, errors)
			}
		}
	} else {
		errors = validateStruct(errors, obj)
		if validator, ok := obj.(Validator); ok {
			errors = validator.Validate(vd.req, errors)
		}
	}
	return errors
}

// Performs required field checking on a struct
func validateStruct(errors Errors, obj interface{}) Errors {
	typ := reflect.TypeOf(obj)
	val := reflect.ValueOf(obj)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Allow ignored fields in the struct
		if field.Tag.Get("form") == "-" || !val.Field(i).CanInterface() {
			continue
		}

		fieldVal := val.Field(i)
		fieldValue := fieldVal.Interface()
		zero := reflect.Zero(field.Type).Interface()

		// Validate nested and embedded structs (if pointer, only do so if not nil)
		if field.Type.Kind() == reflect.Struct ||
			(field.Type.Kind() == reflect.Ptr && !reflect.DeepEqual(zero, fieldValue) &&
				field.Type.Elem().Kind() == reflect.Struct) {
			errors = validateStruct(errors, fieldValue)
		}

	VALIDATE_RULES:
		for _, rule := range strings.Split(field.Tag.Get("binding"), ";") {
			if len(rule) == 0 {
				continue
			}

			switch {
			case rule == "Required":
				if reflect.DeepEqual(zero, fieldValue) {
					errors.Add([]string{field.Name}, ERR_REQUIRED, "Required")
					break VALIDATE_RULES
				}
			case rule == "AlphaDash":
				if alphaDashPattern.MatchString(fmt.Sprintf("%v", fieldValue)) {
					errors.Add([]string{field.Name}, ERR_ALPHA_DASH, "AlphaDash")
					break VALIDATE_RULES
				}
			case rule == "AlphaDashDot":
				if alphaDashDotPattern.MatchString(fmt.Sprintf("%v", fieldValue)) {
					errors.Add([]string{field.Name}, ERR_ALPHA_DASH_DOT, "AlphaDashDot")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "MinSize("):
				min, _ := strconv.Atoi(rule[8 : len(rule)-1])
				if str, ok := fieldValue.(string); ok && utf8.RuneCountInString(str) < min {
					errors.Add([]string{field.Name}, ERR_MIN_SIZE, "MinSize")
					break VALIDATE_RULES
				}
				v := reflect.ValueOf(fieldValue)
				if v.Kind() == reflect.Slice && v.Len() < min {
					errors.Add([]string{field.Name}, ERR_MIN_SIZE, "MinSize")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "MaxSize("):
				max, _ := strconv.Atoi(rule[8 : len(rule)-1])
				if str, ok := fieldValue.(string); ok && utf8.RuneCountInString(str) > max {
					errors.Add([]string{field.Name}, ERR_MAX_SIZE, "MaxSize")
					break VALIDATE_RULES
				}
				v := reflect.ValueOf(fieldValue)
				if v.Kind() == reflect.Slice && v.Len() > max {
					errors.Add([]string{field.Name}, ERR_MAX_SIZE, "MaxSize")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "Range("):
				nums := strings.Split(rule[6:len(rule)-1], ",")
				if len(nums) != 2 {
					break VALIDATE_RULES
				}
				val := com.StrTo(fmt.Sprintf("%v", fieldValue)).MustInt()
				if val < com.StrTo(nums[0]).MustInt() || val > com.StrTo(nums[1]).MustInt() {
					errors.Add([]string{field.Name}, ERR_RANGE, "Range")
					break VALIDATE_RULES
				}
			case rule == "Email":
				if !emailPattern.MatchString(fmt.Sprintf("%v", fieldValue)) {
					errors.Add([]string{field.Name}, ERR_EMAIL, "Email")
					break VALIDATE_RULES
				}
			case rule == "Url":
				str := fmt.Sprintf("%v", fieldValue)
				if len(str) == 0 {
					continue
				} else if !urlPattern.MatchString(str) {
					errors.Add([]string{field.Name}, ERR_URL, "Url")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "In("):
				if !in(fieldValue, rule[3:len(rule)-1]) {
					errors.Add([]string{field.Name}, ERR_IN, "In")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "NotIn("):
				if in(fieldValue, rule[6:len(rule)-1]) {
					errors.Add([]string{field.Name}, ERR_NOT_INT, "NotIn")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "Include("):
				if !strings.Contains(fmt.Sprintf("%v", fieldValue), rule[8:len(rule)-1]) {
					errors.Add([]string{field.Name}, ERR_INCLUDE, "Include")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "Exclude("):
				if strings.Contains(fmt.Sprintf("%v", fieldValue), rule[8:len(rule)-1]) {
					errors.Add([]string{field.Name}, ERR_EXCLUDE, "Exclude")
					break VALIDATE_RULES
				}
			case strings.HasPrefix(rule, "Default("):
				if reflect.DeepEqual(zero, fieldValue) {
					if fieldVal.CanAddr() {
						setWithProperType(field.Type.Kind(), rule[8:len(rule)-1], fieldVal, field.Tag.Get("form"), errors)
					} else {
						errors.Add([]string{field.Name}, ERR_EXCLUDE, "Default")
						break VALIDATE_RULES
					}
				}
			default:
				// Apply custom validation rules.
				for i := range ruleMapper {
					if ruleMapper[i].IsMatch(rule) && !ruleMapper[i].IsValid(errors, field.Name, fieldValue) {
						break VALIDATE_RULES
					}
				}
			}
		}
	}
	return errors
}
