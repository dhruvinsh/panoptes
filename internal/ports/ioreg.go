package ports

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ioregNode = regexp.MustCompile(`^([ |]*)\+-o (.*?)\s+<class ([A-Za-z0-9_]+)`)
	ioregProp = regexp.MustCompile(`^"([^"]+)" = (.*)$`)
)

type ioregNodeT struct {
	col   int
	class string
	props map[string]string
}

// ParseIoreg parses the output of `ioreg -r -c IOUSBHostDevice -l -w0` and maps each
// serial callout device (/dev/cu.*) to the USB device it hangs off.
//
// ioreg prints a tree: node lines ("+-o Name@loc  <class X, ...>") whose indentation
// gives their depth, each followed by its property block ("key" = value). A serial port
// is an IOSerialBSDClient node with an "IOCalloutDevice" property; its VID/PID are on the
// nearest enclosing USB device node.
func ParseIoreg(out string) map[string]Port {
	res := map[string]Port{}
	var stack []*ioregNodeT
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimRight(raw, "\r")
		if m := ioregNode.FindStringSubmatchIndex(line); m != nil {
			col := m[3] // end of the indentation prefix = column of "+-o"
			class := line[m[6]:m[7]]
			for len(stack) > 0 && stack[len(stack)-1].col >= col {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, &ioregNodeT{col: col, class: class, props: map[string]string{}})
			continue
		}
		if len(stack) == 0 {
			continue
		}
		body := strings.TrimLeft(line, " |")
		pm := ioregProp.FindStringSubmatch(body)
		if pm == nil {
			continue
		}
		cur := stack[len(stack)-1]
		val := strings.TrimSpace(pm[2])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		cur.props[pm[1]] = val
		if pm[1] != "IOCalloutDevice" || !strings.HasPrefix(val, "/dev/") {
			continue
		}
		p := Port{Device: val, Description: "n/a", HWID: "n/a"}
		if n := usbAncestor(stack); n != nil {
			vid, pid := n.props["idVendor"], n.props["idProduct"]
			v, _ := strconv.ParseInt(vid, 0, 32)
			d, _ := strconv.ParseInt(pid, 0, 32)
			p.VID, p.PID = int(v), int(d)
			p.Description = firstNonEmpty(n.props["USB Product Name"], n.props["kUSBProductString"],
				n.props["Product Name"], "n/a")
			p.SerialNumber = firstNonEmpty(n.props["USB Serial Number"], n.props["kUSBSerialNumberString"])
			p.HWID = fmt.Sprintf("USB VID:PID=%04X:%04X", p.VID, p.PID)
			if p.SerialNumber != "" {
				p.HWID += " SER=" + p.SerialNumber
			}
		}
		if _, dup := res[val]; !dup {
			res[val] = p
		}
	}
	return res
}

// usbAncestor is the nearest enclosing USB device node (interfaces also carry
// idVendor/idProduct, but not the product name), else any node with a VID/PID.
func usbAncestor(stack []*ioregNodeT) *ioregNodeT {
	has := func(n *ioregNodeT) bool {
		_, v := n.props["idVendor"]
		_, p := n.props["idProduct"]
		return v && p
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if c := stack[i].class; (c == "IOUSBHostDevice" || c == "IOUSBDevice") && has(stack[i]) {
			return stack[i]
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if has(stack[i]) {
			return stack[i]
		}
	}
	return nil
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
