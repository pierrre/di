package di

import (
	"context"
	"fmt"
)

func Example() {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (*myService, Close, error) {
		return &myService{}, nil, nil
	})
	s := MustGet[*myService](ctx, ctn, "")
	s.myMethod()
	// Output:
	// myService.myMethod
}

type myService struct{}

func (s *myService) myMethod() {
	fmt.Println("myService.myMethod")
}
