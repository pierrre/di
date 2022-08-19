//nolint:forbidigo // Allow to use fmt.Println in this test.
package di

import (
	"fmt"
	"io"
	"testing"
)

func Example() {
	// New container.
	c := new(Container)
	// Set ServiceA.
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	// Set ServiceB.
	somethingWrong := false
	Set(c, "", func(c *Container) (*serviceB, Close, error) {
		// We know that ServiceA's builder doesn't return an error, so we ignore it.
		sa := Must(Get[*serviceA](c, ""))
		if somethingWrong {
			return nil, nil, fmt.Errorf("error")
		}
		sb := &serviceB{
			sa.DoA,
		}
		return sb, sb.close, nil
	})
	// Set ServiceC.
	Set(c, "", func(c *Container) (*serviceC, Close, error) {
		sb, err := Get[*serviceB](c, "")
		if err != nil {
			return nil, nil, err
		}
		sc := &serviceC{
			sb.DoB,
		}
		// The ServiceC close function doesn't return an error, so we wrap it.
		cl := func() error {
			sc.close()
			return nil
		}
		return sc, cl, nil
	})
	// Get ServiceC and call it.
	sc, err := Get[*serviceC](c, "")
	if err != nil {
		panic(err)
	}
	sc.DoC()
	// Close container.
	c.Close(func(err error) {
		panic(err)
	})
	// Output:
	// do A
	// do B
	// do C
	// close B
	// close C
}

type serviceA struct{}

func (sa *serviceA) DoA() {
	fmt.Println("do A")
}

type serviceB struct {
	sa func()
}

func (sb *serviceB) DoB() {
	sb.sa()
	fmt.Println("do B")
}

func (sb *serviceB) close() error {
	fmt.Println("close B")
	return nil
}

type serviceC struct {
	sb func()
}

func (sc *serviceC) DoC() {
	sc.sb()
	fmt.Println("do C")
}

func (sc *serviceC) close() {
	fmt.Println("close C")
}

var benchmarkGetServiceNameResult string

func BenchmarkGetServiceNameString(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = getServiceName[string]()
	}
	benchmarkGetServiceNameResult = s
}

func BenchmarkGetServiceNameIOWriter(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = getServiceName[io.Writer]()
	}
	benchmarkGetServiceNameResult = s
}
