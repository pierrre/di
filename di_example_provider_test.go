package di

import (
	"context"
	"fmt"
)

func Example_container() {
	ctx := context.Background()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		fmt.Println("build")
		return "test", nil, nil
	})
	p := ctn.Provider[string]("")
	s := p.MustGet(ctx)
	fmt.Println(s)
	// Output:
	// build
	// test
}
