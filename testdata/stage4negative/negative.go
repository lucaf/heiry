package stage4negative

import "github.com/lucaf/heiry/testdata/stage4boundary"

func privateCall(object stage4boundary.Public) string { return object.Secret() }
