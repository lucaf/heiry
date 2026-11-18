package heiry

import (
	"errors"
	"sync"
	"testing"
)

func TestM7ConcurrentFirstUsePublishesOnePlan(t *testing.T) {
	d := m5Stage3Definition()
	const count = 24
	plans := make([]*Plan[stage3C], count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := range plans {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			leaf := &stage3C{}
			leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
			if err := d.Init(leaf, d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf)); err != nil {
				t.Errorf("lazy Init: %v", err)
				return
			}
			if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
				t.Errorf("Read = %d", got)
			}
			plan, err := d.Prepare()
			if err != nil {
				t.Errorf("warm Prepare: %v", err)
				return
			}
			plans[index] = plan
		}()
	}
	close(start)
	workers.Wait()
	for index, plan := range plans {
		if plan == nil || plan != plans[0] {
			t.Fatalf("plan %d = %p, first = %p", index, plan, plans[0])
		}
	}
}

func TestM7ConcurrentFailedPreparationRetriesAfterRepair(t *testing.T) {
	d := Define[m4A]()
	level := d.Level[m4A, m4APublic, m4APrivate]("")
	const count = 16
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() {
			<-start
			if plan, err := d.Prepare(); plan != nil || !errors.Is(err, &Error{Code: MissingMethods}) {
				t.Errorf("invalid Prepare = (%p, %v)", plan, err)
			}
		})
	}
	close(start)
	workers.Wait()
	level.Bind[m4Read]((*m4A)(nil), ".").Bind[m4Secret]((*m4A)(nil), ".")
	plan, err := d.Prepare()
	if err != nil || plan == nil {
		t.Fatalf("repaired Prepare = (%p, %v)", plan, err)
	}
	if warm, err := d.Prepare(); err != nil || warm != plan {
		t.Fatalf("warm Prepare = (%p, %v)", warm, err)
	}
}

func TestM7FrozenEditCanInterveneBeforeWaitingPrepare(t *testing.T) {
	d := m5Stage3Definition()
	plan := d.MustPrepare()
	start := make(chan struct{})
	var workers sync.WaitGroup
	var warm *Plan[stage3C]
	var warmErr error
	workers.Go(func() {
		<-start
		warm, warmErr = d.Prepare()
	})
	workers.Go(func() {
		<-start
		d.Level[stage3C, stage5Public, contractEmpty]("stage3B")
	})
	close(start)
	workers.Wait()
	if warmErr == nil {
		if warm != plan {
			t.Fatalf("warm Plan = %p, want %p", warm, plan)
		}
	} else if warm != nil || !errors.Is(warmErr, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("interleaved Prepare = (%p, %v)", warm, warmErr)
	}
	if result, err := d.Prepare(); result != nil || !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("later Prepare = (%p, %v)", result, err)
	}
	leaf := &stage3C{}
	if err := plan.Init(leaf, plan.NodeProvider(&leaf.stage3A), plan.NodeProvider(&leaf.stage3B), plan.NodeProvider(leaf)); err != nil {
		t.Fatalf("retained Plan unusable: %v", err)
	}
}

func TestM7ConcurrentValidationAndReferencesSeeCoherentDefinition(t *testing.T) {
	d := m5Stage3Definition()
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Go(func() {
		<-start
		if err := d.Validate(); err != nil {
			t.Errorf("Validate: %v", err)
		}
	})
	workers.Go(func() {
		<-start
		if _, err := d.Prepare(); err != nil {
			t.Errorf("Prepare: %v", err)
		}
	})
	workers.Go(func() {
		<-start
		leaf := &stage3C{}
		refs := []ProviderRef[stage3C]{d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf)}
		if err := d.Init(leaf, refs...); err != nil {
			t.Errorf("Init: %v", err)
		}
	})
	close(start)
	workers.Wait()
}

func TestM7PreparationMaySeePartialConfiguration(t *testing.T) {
	d := Define[m4A]()
	level := d.Level[m4A, m4APublic, m4APrivate]("").Bind[m4Read]((*m4A)(nil), ".")
	start := make(chan struct{})
	var workers sync.WaitGroup
	var plan *Plan[m4A]
	var preparationErr error
	workers.Go(func() {
		<-start
		plan, preparationErr = d.Prepare()
	})
	workers.Go(func() {
		<-start
		level.Bind[m4Secret]((*m4A)(nil), ".")
	})
	close(start)
	workers.Wait()
	if plan == nil {
		if !errors.Is(preparationErr, &Error{Code: MissingMethods}) {
			t.Fatalf("partial Prepare = (%p, %v)", plan, preparationErr)
		}
		if repaired, err := d.Prepare(); err != nil || repaired == nil {
			t.Fatalf("retry = (%p, %v)", repaired, err)
		}
		return
	}
	if preparationErr != nil {
		t.Fatalf("prepared Plan and error: %v", preparationErr)
	}
	if next, err := d.Prepare(); err != nil && !errors.Is(err, &Error{Code: InvalidDeclaration}) || err == nil && next != plan {
		t.Fatalf("after interleaving = (%p, %v)", next, err)
	}
}
