// +build linux

package netutil

import (
	"fmt"
)

var errNoDefaultRoute = fmt.Errorf("could not find default route")
var errNoDefaultHost = fmt.Errorf("could not find default host")
var errNoDefaultInterface = fmt.Errorf("could not find default interface")

func GetDefaultHost() (string, error) {
	//
	// rmsgs, rerr := getDefaultRoutes()
	// if rerr != nil {
	// 	return "", rerr
	// }

	// // prioritize IPv4
	// if rmsg, ok := rmsgs[syscall.AF_INET]; ok {
	// 	if host, err := chooseHost(syscall.AF_INET, rmsg); host != "" || err != nil {
	// 		return host, err
	// 	}
	// 	delete(rmsgs, syscall.AF_INET)
	// }

	// // sort so choice is deterministic
	// var families []int
	// for family := range rmsgs {
	// 	families = append(families, int(family))
	// }
	// sort.Ints(families)

	// for _, f := range families {
	// 	family := uint8(f)
	// 	if host, err := chooseHost(family, rmsgs[family]); host != "" || err != nil {
	// 		return host, err
	// 	}
	// }

	//return "", errNoDefaultHost
	return "", nil
}

// func getDefaultRoutes() (map[uint8]*syscall.NetlinkMessage, error) {
// 	dat, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_UNSPEC)
// 	if err != nil {
// 		return nil, err
// 	}

// 	msgs, msgErr := syscall.ParseNetlinkMessage(dat)
// 	if msgErr != nil {
// 		return nil, msgErr
// 	}

// 	routes := make(map[uint8]*syscall.NetlinkMessage)
// 	rtmsg := syscall.RtMsg{}
// 	for _, m := range msgs {
// 		if m.Header.Type != syscall.RTM_NEWROUTE {
// 			continue
// 		}
// 		buf := bytes.NewBuffer(m.Data[:syscall.SizeofRtMsg])
// 		if rerr := binary.Read(buf, cpuutil.ByteOrder(), &rtmsg); rerr != nil {
// 			continue
// 		}
// 		if rtmsg.Dst_len == 0 && rtmsg.Table == syscall.RT_TABLE_MAIN {
// 			// zero-length Dst_len implies default route
// 			msg := m
// 			routes[rtmsg.Family] = &msg
// 		}
// 	}

// 	if len(routes) > 0 {
// 		return routes, nil
// 	}

// 	return nil, errNoDefaultRoute
// }

// 第一行
// +build 代表编译选项
// linux 代表linux平台
// mac属于非linux平台
