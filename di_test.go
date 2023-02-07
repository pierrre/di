//nolint:forbidigo // Allow to use fmt.Println in this test.
package di

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/pierrre/assert"
	"github.com/pierrre/assert/ext/davecghspew"
	"github.com/pierrre/assert/ext/pierrrecompare"
	"github.com/pierrre/assert/ext/pierrreerrors"
)

func init() {
	pierrrecompare.Configure()
	davecghspew.ConfigureDefault()
	pierrreerrors.Configure()
}

func Test(t *testing.T) {
	c := new(Container)
	builderCallCount := 0
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		builderCallCount++
		return &serviceA{}, nil, nil
	})
	sa, err := Get[*serviceA](c, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	sa, err = Get[*serviceA](c, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	assert.Equal(t, builderCallCount, 1)
}

func TestSetPanicAlreadySet(t *testing.T) {
	c := new(Container)
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	assert.Panics(t, func() {
		Set(c, "", func(c *Container) (*serviceA, Close, error) {
			return &serviceA{}, nil, nil
		})
	})
}

func TestGetErrorNotRegistered(t *testing.T) {
	c := new(Container)
	_, err := Get[*serviceA](c, "")
	assert.Error(t, err)
}

func TestGetErrorType(t *testing.T) {
	c := new(Container)
	Set(c, "test", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	_, err := Get[*serviceB](c, "test")
	assert.Error(t, err)
}

func TestGetErrorBuilder(t *testing.T) {
	c := new(Container)
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return nil, nil, errors.New("error")
	})
	_, err := Get[*serviceA](c, "")
	assert.Error(t, err)
}

func TestClose(t *testing.T) {
	c := new(Container)
	var closeServiceCalled bool
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, func() error {
			closeServiceCalled = true
			return nil
		}, nil
	})
	_, err := Get[*serviceA](c, "")
	assert.NoError(t, err)
	c.Close(func(err error) {
		assert.NoError(t, err)
	})
	assert.True(t, closeServiceCalled)
}

func TestCloseNil(t *testing.T) {
	c := new(Container)
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	_, err := Get[*serviceA](c, "")
	assert.NoError(t, err)
	c.Close(func(err error) {
		assert.NoError(t, err)
	})
}

func TestCloseError(t *testing.T) {
	c := new(Container)
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, func() error {
			return errors.New("error")
		}, nil
	})
	_, err := Get[*serviceA](c, "")
	assert.NoError(t, err)
	c.Close(func(err error) {
		assert.Error(t, err)
	})
}

func TestMust(t *testing.T) {
	Must("", nil)
}

func TestMustPanic(t *testing.T) {
	assert.Panics(t, func() {
		Must("", errors.New("error"))
	})
}

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
