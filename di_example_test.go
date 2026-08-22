package di

import (
	"context"
	"fmt"
)

func Example() {
	ctx := context.Background()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (*myService, Close, error) {
		return &myService{}, nil, nil
	})
	s := ctn.MustGet[*myService](ctx, "")
	s.myMethod()
	// Output:
	// myService.myMethod
}

type myService struct{}

func (s *myService) myMethod() {
	fmt.Println("myService.myMethod")
}
