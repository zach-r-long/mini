package mm

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"phenix/internal/mm/mmcli"
	"phenix/types"
)

type Minimega struct{}

func (Minimega) ReadScriptFromFile(filename string) error {
	cmd := mmcli.NewCommand()
	cmd.Command = "read " + filename

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("reading mmcli script: %w", err)
	}

	return nil
}

func (Minimega) ClearNamespace(ns string) error {
	cmd := mmcli.NewCommand()
	cmd.Command = "clear namespace " + ns

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("clearing minimega namespace: %w", err)
	}

	return nil
}

func (Minimega) LaunchVMs(ns string) error {
	cmd := mmcli.NewCommand()
	cmd.Command = "vm launch"

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("launching VMs: %w", err)
	}

	cmd.Command = "vm start all"

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("starting VMs: %w", err)
	}

	return nil
}

func (this Minimega) GetVMInfo(opts ...Option) types.VMs {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = "vm info"
	cmd.Columns = []string{"host", "name", "state", "uptime", "vlan", "tap"}

	if o.vm != "" {
		cmd.Filters = []string{"name=" + o.vm}
	}

	var vms types.VMs

	for _, row := range mmcli.RunTabular(cmd) {
		var vm types.VM

		vm.Host = row["host"]
		vm.Name = row["name"]
		vm.Running = row["state"] == "RUNNING"

		s := row["vlan"]
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")

		vm.Networks = strings.Split(s, ", ")

		s = row["tap"]
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")

		vm.Taps = strings.Split(s, ", ")
		vm.Captures = this.GetVMCaptures(opts...)

		uptime, err := time.ParseDuration(row["uptime"])
		if err == nil {
			vm.Uptime = uptime.Seconds()
		}

		vms = append(vms, vm)
	}

	return vms
}

func (Minimega) StartVM(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("vm start %s", o.vm)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("starting VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	return nil
}

func (Minimega) StopVM(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("vm stop %s", o.vm)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("stopping VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	return nil
}

func (Minimega) RedeployVM(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)

	cmd.Command = "vm config clone " + o.vm
	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("cloning VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	cmd.Command = "clear vm config migrate"
	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("clearing config for VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	cmd.Command = "vm kill " + o.vm
	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("killing VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	if err := flush(o.ns); err != nil {
		return err
	}

	if o.cpu != 0 {
		cmd.Command = fmt.Sprintf("vm config vcpus %d", o.cpu)

		if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
			return fmt.Errorf("configuring VCPUs for VM %s in namespace %s: %w", o.vm, o.ns, err)
		}
	}

	if o.mem != 0 {
		cmd.Command = fmt.Sprintf("vm config mem %d", o.mem)

		if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
			return fmt.Errorf("configuring memory for VM %s in namespace %s: %w", o.vm, o.ns, err)
		}
	}

	if o.disk != "" {
		var disk string

		if len(o.injects) == 0 {
			disk = o.disk
		} else {
			cmd.Command = "vm config disk"
			cmd.Columns = []string{"disks"}
			cmd.Filters = []string{"name=" + o.vm}

			config := mmcli.RunTabular(cmd)

			cmd.Columns = nil
			cmd.Filters = nil

			if len(config) == 0 {
				return fmt.Errorf("disk config not found for VM %s in namespace %s", o.vm, o.ns)
			}

			// Should only be one row of data since we filter by VM name above.
			status := config[0]

			disk = filepath.Base(status["disks"])

			if strings.Contains(disk, "_snapshot") {
				cmd.Command = fmt.Sprintf("disk snapshot %s %s", o.disk, disk)

				if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
					return fmt.Errorf("snapshotting disk for VM %s in namespace %s: %w", o.vm, o.ns, err)
				}

				if err := inject(disk, o.injectPart, o.injects...); err != nil {
					return err
				}
			} else {
				disk = o.disk
			}
		}

		cmd.Command = "vm config disk " + disk

		if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
			return fmt.Errorf("configuring disk for VM %s in namespace %s: %w", o.vm, o.ns, err)
		}
	}

	cmd.Command = "vm launch kvm " + o.vm
	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("scheduling VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	cmd.Command = "vm launch"
	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("launching scheduled VMs in namespace %s: %w", o.ns, err)
	}

	cmd.Command = fmt.Sprintf("vm start %s", o.vm)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("starting VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	return nil
}

func (Minimega) KillVM(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("vm kill %s", o.vm)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("killing VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	return flush(o.ns)
}

func (Minimega) ConnectVMInterface(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("vm net connect %s %d %s", o.vm, o.connectIface, o.connectVLAN)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("connecting interface %d on VM %s to VLAN %s in namespace %s: %w", o.connectIface, o.vm, o.connectVLAN, o.ns, err)
	}

	return nil
}

func (Minimega) DisonnectVMInterface(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("vm net disconnect %s %d", o.vm, o.connectIface)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("disconnecting interface %d on VM %s in namespace %s: %w", o.connectIface, o.vm, o.ns, err)
	}

	return nil
}

func (Minimega) StartVMCapture(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("capture pcap vm %s %d %s", o.vm, o.captureIface, o.captureFile)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("starting VM capture for interface %d on VM %s in namespace %s: %w", o.captureIface, o.vm, o.ns, err)
	}

	return nil
}

func (Minimega) StopVMCapture(opts ...Option) error {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = fmt.Sprintf("capture pcap delete vm %s", o.vm)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("deleting VM captures for VM %s in namespace %s: %w", o.vm, o.ns, err)
	}

	return nil
}

func (Minimega) GetExperimentCaptures(opts ...Option) []types.Capture {
	o := NewOptions(opts...)

	cmd := mmcli.NewNamespacedCommand(o.ns)
	cmd.Command = "capture"
	cmd.Columns = []string{"interface", "path"}

	var captures []types.Capture

	for _, row := range mmcli.RunTabular(cmd) {
		// `interface` column will be in the form of <vm_name>:<iface_idx>
		iface := strings.Split(row["interface"], ":")

		vm := iface[0]
		idx, _ := strconv.Atoi(iface[1])

		capture := types.Capture{
			VM:        vm,
			Interface: idx,
			Filepath:  row["path"],
		}

		captures = append(captures, capture)
	}

	return captures
}

func (this Minimega) GetVMCaptures(opts ...Option) []types.Capture {
	o := NewOptions(opts...)

	var (
		captures = this.GetExperimentCaptures(opts...)
		keep     []types.Capture
	)

	for _, capture := range captures {
		if capture.VM == o.vm {
			keep = append(keep, capture)
		}
	}

	return keep
}

func flush(ns string) error {
	cmd := mmcli.NewNamespacedCommand(ns)
	cmd.Command = "vm flush"

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("flushing VMs in namespace %s: %w", ns, err)
	}

	return nil
}

func inject(disk string, part int, injects ...string) error {
	files := strings.Join(injects, " ")

	cmd := mmcli.NewCommand()
	cmd.Command = fmt.Sprintf("disk inject %s:%d files %s", disk, part, files)

	if err := mmcli.ErrorResponse(mmcli.Run(cmd)); err != nil {
		return fmt.Errorf("injecting files into disk %s: %w", disk, err)
	}

	return nil
}
