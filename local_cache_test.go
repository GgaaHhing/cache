package cache

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// TestClose 不需要知道是否close了，只要有重复Close，就说明执行了Close
func TestClose(t *testing.T) {
	c := NewBuildInMapCache(100, 2*time.Second)
	_ = c.Close()
	err := c.Close()
	assert.Equal(t, "cache：Close重复关闭", err.Error())
}
