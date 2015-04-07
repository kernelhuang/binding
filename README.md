binding [![Build Status](https://drone.io/github.com/tango-contrib/binding/status.png)](https://drone.io/github.com/tango-contrib/binding/latest) [![](http://gocover.io/_badge/github.com/tango-contrib/binding)](http://gocover.io/github.com/tango-contrib/binding)
=======

Middlware binding provides request data binding and validation for [Tango](https://github.com/lunny/tango).

## Installation

	go get github.com/tango-contrib/binding

## Example

```Go
import (
    "github.com/lunny/tango"
    "github.com/tango-contrib/binding"
)

type Action struct {
    binding.Binder
}

type MyStruct struct {
    Id int64
    Name string
}

func (a *Action) Get() string {
    var mystruct MyStruct
    errs := a.Bind(&mystruct)
    return fmt.Sprintf("%v, %v", mystruct, errs)
}

func main() {
    t := tango.Classic()
    t.Use(binding.Bind())
    t.Get("/", new(Action))
    t.Run()
}
```

Visit `/?id=1&name=2` on your browser and you will find output
```
{1 sss}, []
```

## Getting Help

- [API Reference](https://gowalker.org/github.com/tango-contrib/binding)

## Credits

This package is forked from [macaron-contrib/binding](https://github.com/macaron-contrib/binding) with modifications.

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.