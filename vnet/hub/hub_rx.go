package hub

import (
	"runtime"

	tun "github.com/sagernet/sing-tun"
)

func (h *Hub) rxLoop() {
	switch runtime.GOOS {
	case "linux":
		h.writeDeviceLinux()
	default:
	}
	h.writeDevice()
}

func (h *Hub) writeDeviceLinux() {
	h.Infof("batch write device loop started")
	dev := h.dev.(tun.LinuxTUN)
	for event := range h.rxEventChan {
		_, err := dev.BatchWrite(event.buf[:event.n], h.cfg.HeaderSize)
		if err != nil {
			h.Stop(err.Error())
			return
		}
	}
}

func (h *Hub) writeDevice() {
	h.Infof("write device loop started")
	for event := range h.rxEventChan {
		for i := 0; i < event.n; i++ {
			_, err := h.dev.Write(event.buf[i][:event.sizes[i]])
			if err != nil {
				h.Stop(err.Error())
				return
			}
		}
	}
}
