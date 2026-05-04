package validation

import (
	"fmt"
	"sync"
	"testing"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestValidatorCache_ConcurrentSetGet(t *testing.T) {
	c := NewValidatorCache(0)

	const goroutines = 20
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			tenantID := fmt.Sprintf("tenant-%d", g%5)
			for i := range iterations {
				validators := map[string]*FieldValidator{
					fmt.Sprintf("field-%d", i): NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, false, nil),
				}
				c.Set(tenantID, validators)
				c.Get(tenantID)
			}
		}()
	}

	wg.Wait()
}

func TestValidatorCache_ConcurrentSetInvalidate(t *testing.T) {
	c := NewValidatorCache(0)

	const goroutines = 20
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			tenantID := fmt.Sprintf("tenant-%d", g%3)
			for range iterations {
				validators := map[string]*FieldValidator{
					"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, false, nil),
				}
				c.Set(tenantID, validators)
				c.Invalidate(tenantID)
				c.Get(tenantID)
			}
		}()
	}

	wg.Wait()
}
