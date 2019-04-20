package fake

type FakeReaper struct{}

func (f *FakeReaper) Stop() {}

func New() *FakeReaper {
	return &FakeReaper{}
}
