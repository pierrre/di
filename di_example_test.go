package di

import (
	"context"
	"fmt"
)

func Example() {
	ctx := context.Background()
	ctn := new(Container)
	defer ctn.Close(ctx)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		fmt.Println("start build A")
		defer fmt.Println("end build A")
		return &serviceA{
			b: MustGet[*serviceB](ctx, ctn, "").doB,
			c: MustGet[*serviceC](ctx, ctn, "").doC,
		}, nil, nil
	})
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (*serviceB, Close, error) {
		fmt.Println("start build B")
		defer fmt.Println("end build B")
		sb := &serviceB{}
		return sb, sb.close, nil
	})
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (*serviceC, Close, error) {
		fmt.Println("start build C")
		defer fmt.Println("end build C")
		return &serviceC{}, nil, nil
	})
	fmt.Println("configured")
	sa := MustGet[*serviceA](ctx, ctn, "")
	fmt.Println("initialized")
	sa.doA()
	// Output:
	// configured
	// start build A
	// start build B
	// end build B
	// start build C
	// end build C
	// end build A
	// initialized
	// do B
	// do A
	// do C
	// close B
}

type serviceA struct {
	b func()
	c func()
}

func (sa *serviceA) doA() {
	sa.b()
	fmt.Println("do A")
	sa.c()
}

type serviceB struct{}

func (sb *serviceB) doB() {
	fmt.Println("do B")
}

func (sb *serviceB) close(ctx context.Context) error {
	fmt.Println("close B")
	return nil
}

type serviceC struct{}

func (sc *serviceC) doC() {
	fmt.Println("do C")
}
