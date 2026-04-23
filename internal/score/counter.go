package score

type Counter struct {
	total int
}

func (c *Counter) Add(n int) {
	if n <= 0 {
		return
	}
	c.total += n
}

func (c *Counter) Total() int { return c.total }

func (c *Counter) Reset() { c.total = 0 }
