package system

import (
	"fmt"
)

// cidrToMask converts CIDR notation to subnet mask
func cidrToMask(bits int) string {
	m := []int{0, 0, 0, 0}
	for i := 0; i < bits; i++ {
		m[i/8] += 1 << uint(7-i%8)
	}
	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}
