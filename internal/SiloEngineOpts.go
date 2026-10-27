package internal

type SiloEngineOpts struct {
	SessionBaseImageIdOrName string
}

func NewSiloEngineOpts() *SiloEngineOpts {
	return &SiloEngineOpts{}
}

func (se *SiloEngineOpts) WithSessionBaseImageName(imageIdOrName string) *SiloEngineOpts {
	se.SessionBaseImageIdOrName = imageIdOrName
	return se
}
