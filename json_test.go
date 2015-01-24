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

package binding

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/lunny/tango"
	. "github.com/smartystreets/goconvey/convey"
)

var jsonTestCases = []jsonTestCase{
	{
		description:         "Happy path",
		shouldSucceedOnJson: true,
		payload:             `{"title": "Glorious Post Title", "content": "Lorem ipsum dolor sit amet"}`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            Post{Title: "Glorious Post Title", Content: "Lorem ipsum dolor sit amet"},
	},
	{
		description:         "Happy path with interface",
		shouldSucceedOnJson: true,
		withInterface:       true,
		payload:             `{"title": "Glorious Post Title", "content": "Lorem ipsum dolor sit amet"}`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            Post{Title: "Glorious Post Title", Content: "Lorem ipsum dolor sit amet"},
	},
	{
		description:         "Nil payload",
		shouldSucceedOnJson: false,
		payload:             `-nil-`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            Post{},
	},
	{
		description:         "Empty payload",
		shouldSucceedOnJson: false,
		payload:             ``,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            Post{},
	},
	{
		description:         "Empty content type",
		shouldSucceedOnJson: true,
		shouldFailOnBind:    true,
		payload:             `{"title": "Glorious Post Title", "content": "Lorem ipsum dolor sit amet"}`,
		contentType:         ``,
		expected:            Post{Title: "Glorious Post Title", Content: "Lorem ipsum dolor sit amet"},
	},
	{
		description:         "Unsupported content type",
		shouldSucceedOnJson: true,
		shouldFailOnBind:    true,
		payload:             `{"title": "Glorious Post Title", "content": "Lorem ipsum dolor sit amet"}`,
		contentType:         `BoGuS`,
		expected:            Post{Title: "Glorious Post Title", Content: "Lorem ipsum dolor sit amet"},
	},
	{
		description:         "Malformed JSON",
		shouldSucceedOnJson: false,
		payload:             `{"title":"foo"`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            Post{},
	},
	{
		description:         "Deserialization with nested and embedded struct",
		shouldSucceedOnJson: true,
		payload:             `{"title":"Glorious Post Title", "id":1, "author":{"name":"Matt Holt"}}`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            BlogPost{Post: Post{Title: "Glorious Post Title"}, Id: 1, Author: Person{Name: "Matt Holt"}},
	},
	{
		description:         "Deserialization with nested and embedded struct with interface",
		shouldSucceedOnJson: true,
		withInterface:       true,
		payload:             `{"title":"Glorious Post Title", "id":1, "author":{"name":"Matt Holt"}}`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            BlogPost{Post: Post{Title: "Glorious Post Title"}, Id: 1, Author: Person{Name: "Matt Holt"}},
	},
	{
		description:         "Required nested struct field not specified",
		shouldSucceedOnJson: false,
		payload:             `{"title":"Glorious Post Title", "id":1, "author":{}}`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            BlogPost{Post: Post{Title: "Glorious Post Title"}, Id: 1},
	},
	{
		description:         "Required embedded struct field not specified",
		shouldSucceedOnJson: false,
		payload:             `{"id":1, "author":{"name":"Matt Holt"}}`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            BlogPost{Id: 1, Author: Person{Name: "Matt Holt"}},
	},
	{
		description:         "Slice of Posts",
		shouldSucceedOnJson: true,
		payload:             `[{"title": "First Post"}, {"title": "Second Post"}]`,
		contentType:         _JSON_CONTENT_TYPE,
		expected:            []Post{Post{Title: "First Post"}, Post{Title: "Second Post"}},
	},
}

func Test_Json(t *testing.T) {
	Convey("Test JSON", t, func() {
		for _, testCase := range jsonTestCases {
			performJsonTest(t, testCase)
		}
	})
}

/*
var obj interface{}
var errors Errors
*/
type JsonAction struct {
	Binder
}

func (v *JsonAction) Get() error {
	return v.Post()
}

func (v *JsonAction) Post() error {
	errors = v.Json(obj)
	if errors.Len() > 0 {
		return fmt.Errorf("%+v", errors)
	}
	return nil
}

func performJsonTest(t *testing.T, testCase jsonTestCase) {
	var payload io.Reader
	httpRecorder := httptest.NewRecorder()
	m := tango.Classic()
	m.Use(Bind())

	jsonTestHandler := func(actual interface{}, errs Errors) {
		if testCase.shouldSucceedOnJson && len(errs) > 0 {
			So(len(errs), ShouldEqual, 0)
		} else if !testCase.shouldSucceedOnJson && len(errs) == 0 {
			So(len(errs), ShouldNotEqual, 0)
		}
		So(fmt.Sprintf("%+v", actual), ShouldEqual, fmt.Sprintf("%+v", testCase.expected))
	}

	obj = reflect.New(reflect.TypeOf(testCase.expected)).Interface()

	switch testCase.expected.(type) {
	case []Post:
		if testCase.withInterface {
			m.Post(testRoute, new(JsonAction))
		} else {
			m.Post(testRoute, new(JsonAction))
		}
	case Post:
		if testCase.withInterface {
			m.Post(testRoute, new(JsonAction))
		} else {
			m.Post(testRoute, new(JsonAction))
		}

	case BlogPost:
		if testCase.withInterface {
			m.Post(testRoute, new(JsonAction))
		} else {
			m.Post(testRoute, new(JsonAction))
		}
	}

	if testCase.payload == "-nil-" {
		payload = nil
	} else {
		payload = strings.NewReader(testCase.payload)
	}

	req, err := http.NewRequest("POST", testRoute, payload)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", testCase.contentType)

	m.ServeHTTP(httpRecorder, req)

	switch httpRecorder.Code {
	case http.StatusNotFound:
		if testCase.shouldSucceedOnJson {
			panic("Routing is messed up in test fixture (got 404): check method and path on '" + testCase.description + "'")
		}
	case http.StatusInternalServerError:
		if testCase.shouldSucceedOnJson {
			panic("Something bad happened on '" + testCase.description + "'")
		}
	default:
		if testCase.shouldSucceedOnJson &&
			httpRecorder.Code != http.StatusOK &&
			!testCase.shouldFailOnBind {
			So(httpRecorder.Code, ShouldEqual, http.StatusOK)
		}
	}

	jsonTestHandler(reflect.ValueOf(obj).Elem().Interface(), errors)
}

type (
	jsonTestCase struct {
		description         string
		withInterface       bool
		shouldSucceedOnJson bool
		shouldFailOnBind    bool
		payload             string
		contentType         string
		expected            interface{}
	}
)
