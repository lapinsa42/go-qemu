package qemu

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// Machine represents a QEMU virtual machine
type Machine struct {
	Cores  int    // Number of CPU cores
	Memory uint64 // RAM quantity in megabytes

	cd      string
	vnc     string
	monitor string
	drives  []Drive
	ifaces  []NetDev
}

// Drive represents a machine hard drive
type Drive struct {
	Path   string // Image file path
	Format string // Image format
}

// NewMachine creates a new virtual machine
// with the specified number of cpu cores and memory
func NewMachine(cores int, memory uint64) Machine {
	var machine Machine
	machine.Cores = cores
	machine.Memory = memory
	machine.drives = make([]Drive, 0)

	return machine
}

// AddCDRom attaches a disk image
// as a CD-ROM on the machine
func (m *Machine) AddCDRom(dev string) {
	m.cd = dev
}

// AddDrive attaches a new hard drive to
// the virtual machine
func (m *Machine) AddDrive(d Drive) {
	m.drives = append(m.drives, d)
}

// AddDriveImage attaches the specified Image to
// the virtual machine
func (m *Machine) AddDriveImage(img Image) {
	m.drives = append(m.drives, Drive{img.Path, img.Format})
}

// AddNetworkDevice attaches the specified netdev tp
// the virtual machine
func (m *Machine) AddNetworkDevice(netdev NetDev) {
	m.ifaces = append(m.ifaces, netdev)
}

// AddVNC attaches a VNC server to
// the virtual machine, bound to the specified address
func (m *Machine) AddVNC(addr string, port int) {
	m.vnc = fmt.Sprintf("%s:%d", addr, port)
}

// AddMonitor redirects the QEMU monitor
// to the specified unix socket file
func (m *Machine) AddMonitorUnix(dev string) {
	m.monitor = dev
}

// Start stars the machine
// The 'kvm' bool specifies if KVM should be used
// It returns the QEMU process and an error (if any)
func (m *Machine) Start(arch string, kvm bool) (*os.Process, error) {
	qemu := fmt.Sprintf("qemu-system-%s", arch)
	args := []string{"-smp", strconv.Itoa(m.Cores), "-m", strconv.FormatUint(m.Memory, 10)}

	if kvm {
		args = append(args, "-enable-kvm")
	}

	if len(m.cd) > 0 {
		args = append(args, "-cdrom")
		args = append(args, m.cd)
	}

	for _, drive := range m.drives {
		args = append(args, "-drive")
		args = append(args, fmt.Sprintf("file=%s,format=%s", drive.Path, drive.Format))
	}

	if len(m.ifaces) == 0 {
		args = append(args, "-net")
		args = append(args, "none")
	}

	for _, iface := range m.ifaces {
		s := fmt.Sprintf("%s,id=%s", iface.Type, iface.ID)
		if len(iface.IfName) > 0 {
			s = fmt.Sprintf("%s,ifname=%s", s, iface.IfName)
		}

		args = append(args, "-netdev")
		args = append(args, s)

		s = fmt.Sprintf("virtio-net,netdev=%s", iface.ID)
		if len(iface.MAC) > 0 {
			s = fmt.Sprintf("%s,mac=%s", s, iface.MAC)
		}

		args = append(args, "-device")
		args = append(args, s)
	}

	if len(m.vnc) > 0 {
		args = append(args, "-vnc")
		args = append(args, m.vnc)
	}

	if len(m.monitor) > 0 {
		args = append(args, "-qmp")
		args = append(args, fmt.Sprintf("unix:%s,server,nowait", m.monitor))
	}

	cmd := exec.Command(qemu, args...)
	cmd.SysProcAttr = new(syscall.SysProcAttr)
	cmd.SysProcAttr.Setsid = true

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	proc := cmd.Process
	errc := make(chan error)

	go func() {
		err := cmd.Wait()
		if err != nil {
			errc <- fmt.Errorf("'qemu-system-%s': %s", arch, err)
			return
		}
	}()

	time.Sleep(50 * time.Millisecond)

	var vmerr error
	select {
	case vmerr = <-errc:
		if vmerr != nil {
			return nil, vmerr
		}
	default:
		break
	}

	return proc, nil
}
