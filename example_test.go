package heiry_test

import (
	"fmt"

	"github.com/lucaf/heiry"
)

// Each operation has one single-method interface.
type (
	Speaker interface{ Speak() string }
	Namer   interface{ Name() string }
)

// animal is the root level.
type animal struct{ heiry.Node }

// Speak calls Name virtually: the most-derived implementation answers.
func (a *animal) Speak() string {
	return heiry.Method[Namer](a.Node.Scope()).Name() + " makes a sound"
}

func (a *animal) Name() string { return "an animal" }

// dog embeds animal by value and overrides Name only.
type dog struct {
	animal
	heiry.Node
}

// Name extends the implementation below dog's level.
func (d *dog) Name() string {
	return "a dog, which is " + heiry.ParentMethod[Namer](d.Node.Scope()).Name() + ","
}

func Example() {
	// Declare the hierarchy once. Speaker is the public contract and Namer the
	// private one; no object is involved yet.
	hierarchy := heiry.Define[dog]().
		Level[animal, Speaker, Namer]("").
		Bind[Speaker]((*animal)(nil), ".").
		Bind[Namer]((*animal)(nil), ".").End().
		Level[dog, Speaker, Namer]("animal").
		Bind[Namer]((*dog)(nil), ".").End()

	// Allocate an object as usual, then bind it. The first Init also
	// validates the declaration.
	d := &dog{}
	err := hierarchy.Init(d, hierarchy.NodeProvider(&d.animal), hierarchy.NodeProvider(d))
	if err != nil {
		fmt.Println(err)
		return
	}

	// Speak is inherited from animal, yet it reaches dog's Name.
	fmt.Println(heiry.Method[Speaker](d.Node.Scope()).Speak())
	// Output: a dog, which is an animal, makes a sound
}
