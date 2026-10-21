package attachment

import "github.com/cyberinq/agiparel/services/edge-daemon/pkg/attachment"

type SubBundleNode = attachment.SubBundleNode
type AttachmentEngine = attachment.AttachmentEngine

var (
	ErrShadeMismatch   = attachment.ErrShadeMismatch
	ErrCutMismatch     = attachment.ErrCutMismatch
	ErrInvalidSequence = attachment.ErrInvalidSequence
)

func NewAttachmentEngine(modbusAddr ...string) *AttachmentEngine {
	return attachment.NewAttachmentEngine(modbusAddr...)
}