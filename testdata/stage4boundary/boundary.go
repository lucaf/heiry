// Package stage4boundary is a separate consumer of the public wrapper fixture.
package stage4boundary

// Public is the external view of the object.
type Public interface{ Read() string }

// Consume calls the wrapper through its public interface and tests its
// dynamic method set from outside the heiry package.
func Consume(object Public) (string, bool) {
	result := object.Read()
	_, private := any(object).(interface{ Secret() string })
	return result, private
}
