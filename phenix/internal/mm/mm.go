package mm

import "phenix/types"

var DefaultMM = new(Minimega)

type MM interface {
	ReadScriptFromFile(string) error
	ClearNamespace(string) error

	LaunchVMs(string) error

	GetVMInfo(...Option) types.VMs
	StartVM(...Option) error
	StopVM(...Option) error
	RedeployVM(...Option) error
	KillVM(...Option) error

	ConnectVMInterface(...Option) error
	DisconnectVMInterface(...Option) error

	StartVMCapture(...Option) error
	StopVMCapture(...Option) error
	GetExperimentCaptures(...Option) []types.Capture
	GetVMCaptures(...Option) []types.Capture
}
