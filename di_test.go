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
	rec, _ := assert.Panics(t, func() {
		Set(c, "", func(c *Container) (*serviceA, Close, error) {
			return &serviceA{}, nil, nil
		})
	})
	err, _ := assert.Type[error](t, rec)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "*di.serviceA")
	assert.ErrorIs(t, err, ErrAlreadySet)
	assert.ErrorEqual(t, err, "service \"*di.serviceA\": already set")
}

func TestGetErrorNotSet(t *testing.T) {
	c := new(Container)
	_, err := Get[*serviceA](c, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "*di.serviceA")
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service \"*di.serviceA\": not set")
}

func TestGetErrorType(t *testing.T) {
	c := new(Container)
	Set(c, "test", func(c *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	_, err := Get[*serviceB](c, "test")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "test")
	var typeErr *TypeError
	assert.ErrorAs(t, err, &typeErr)
	assert.Equal(t, typeErr.Type, "*di.serviceB")
	assert.ErrorEqual(t, err, "service \"test\": type *di.serviceB does not match")
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
	builderCalled := 0
	closeCalled := 0
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		builderCalled++
		return &serviceA{}, func() error {
			closeCalled++
			return nil
		}, nil
	})
	count := 5
	for i := 0; i < count; i++ {
		_, err := Get[*serviceA](c, "")
		assert.NoError(t, err)
		c.Close(func(err error) {
			assert.NoError(t, err)
		})
	}
	assert.Equal(t, builderCalled, count)
	assert.Equal(t, closeCalled, count)
}

func TestCloseNil(t *testing.T) {
	c := new(Container)
	builderCalled := 0
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		builderCalled++
		return &serviceA{}, nil, nil
	})
	count := 5
	for i := 0; i < count; i++ {
		_, err := Get[*serviceA](c, "")
		assert.NoError(t, err)
		c.Close(func(err error) {
			assert.NoError(t, err)
		})
	}
	assert.Equal(t, builderCalled, count)
}

func TestCloseNotInitialized(t *testing.T) {
	c := new(Container)
	Set(c, "", func(c *Container) (*serviceA, Close, error) {
		return nil, nil, errors.New("error")
	})
	_, err := Get[*serviceA](c, "")
	assert.Error(t, err)
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
		var serviceErr *ServiceError
		assert.ErrorAs(t, err, &serviceErr)
		assert.Equal(t, serviceErr.Name, "*di.serviceA")
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

func TestGetTypeNameString(t *testing.T) {
	s := getTypeName[string]()
	assert.Equal(t, s, "string")
}

func TestGetTypeNameIOWriter(t *testing.T) {
	s := getTypeName[io.Writer]()
	assert.Equal(t, s, "io.Writer")
}

var benchmarkGetServiceNameResult string

func BenchmarkGetTypeNameString(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = getTypeName[string]()
	}
	benchmarkGetServiceNameResult = s
}

func BenchmarkGetTypeNameIOWriter(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = getTypeName[io.Writer]()
	}
	benchmarkGetServiceNameResult = s
}
