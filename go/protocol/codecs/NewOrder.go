// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
)

type NewOrder struct {
	ClientOrderId int64
	Side          SideEnum
	Price         int64
	Qty           int64
}

func (n *NewOrder) Encode(_m *SbeGoMarshaller, _w io.Writer, doRangeCheck bool) error {
	if doRangeCheck {
		if err := n.RangeCheck(n.SbeSchemaVersion(), n.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	if err := _m.WriteInt64(_w, n.ClientOrderId); err != nil {
		return err
	}
	if err := n.Side.Encode(_m, _w); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, n.Price); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, n.Qty); err != nil {
		return err
	}
	return nil
}

func (n *NewOrder) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16, blockLength uint16, doRangeCheck bool) error {
	if !n.ClientOrderIdInActingVersion(actingVersion) {
		n.ClientOrderId = n.ClientOrderIdNullValue()
	} else {
		if err := _m.ReadInt64(_r, &n.ClientOrderId); err != nil {
			return err
		}
	}
	if n.SideInActingVersion(actingVersion) {
		if err := n.Side.Decode(_m, _r, actingVersion); err != nil {
			return err
		}
	}
	if !n.PriceInActingVersion(actingVersion) {
		n.Price = n.PriceNullValue()
	} else {
		if err := _m.ReadInt64(_r, &n.Price); err != nil {
			return err
		}
	}
	if !n.QtyInActingVersion(actingVersion) {
		n.Qty = n.QtyNullValue()
	} else {
		if err := _m.ReadInt64(_r, &n.Qty); err != nil {
			return err
		}
	}
	if actingVersion > n.SbeSchemaVersion() && blockLength > n.SbeBlockLength() {
		io.CopyN(ioutil.Discard, _r, int64(blockLength-n.SbeBlockLength()))
	}
	if doRangeCheck {
		if err := n.RangeCheck(actingVersion, n.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	return nil
}

func (n *NewOrder) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if n.ClientOrderIdInActingVersion(actingVersion) {
		if n.ClientOrderId < n.ClientOrderIdMinValue() || n.ClientOrderId > n.ClientOrderIdMaxValue() {
			return fmt.Errorf("Range check failed on n.ClientOrderId (%v < %v > %v)", n.ClientOrderIdMinValue(), n.ClientOrderId, n.ClientOrderIdMaxValue())
		}
	}
	if err := n.Side.RangeCheck(actingVersion, schemaVersion); err != nil {
		return err
	}
	if n.PriceInActingVersion(actingVersion) {
		if n.Price < n.PriceMinValue() || n.Price > n.PriceMaxValue() {
			return fmt.Errorf("Range check failed on n.Price (%v < %v > %v)", n.PriceMinValue(), n.Price, n.PriceMaxValue())
		}
	}
	if n.QtyInActingVersion(actingVersion) {
		if n.Qty < n.QtyMinValue() || n.Qty > n.QtyMaxValue() {
			return fmt.Errorf("Range check failed on n.Qty (%v < %v > %v)", n.QtyMinValue(), n.Qty, n.QtyMaxValue())
		}
	}
	return nil
}

func NewOrderInit(n *NewOrder) {
	return
}

func (*NewOrder) SbeBlockLength() (blockLength uint16) {
	return 25
}

func (*NewOrder) SbeTemplateId() (templateId uint16) {
	return 1
}

func (*NewOrder) SbeSchemaId() (schemaId uint16) {
	return 901
}

func (*NewOrder) SbeSchemaVersion() (schemaVersion uint16) {
	return 1
}

func (*NewOrder) SbeSemanticType() (semanticType []byte) {
	return []byte("")
}

func (*NewOrder) SbeSemanticVersion() (semanticVersion string) {
	return "1.0"
}

func (*NewOrder) ClientOrderIdId() uint16 {
	return 1
}

func (*NewOrder) ClientOrderIdSinceVersion() uint16 {
	return 0
}

func (n *NewOrder) ClientOrderIdInActingVersion(actingVersion uint16) bool {
	return actingVersion >= n.ClientOrderIdSinceVersion()
}

func (*NewOrder) ClientOrderIdDeprecated() uint16 {
	return 0
}

func (*NewOrder) ClientOrderIdMetaAttribute(meta int) string {
	switch meta {
	case 1:
		return ""
	case 2:
		return ""
	case 3:
		return ""
	case 4:
		return "required"
	}
	return ""
}

func (*NewOrder) ClientOrderIdMinValue() int64 {
	return math.MinInt64 + 1
}

func (*NewOrder) ClientOrderIdMaxValue() int64 {
	return math.MaxInt64
}

func (*NewOrder) ClientOrderIdNullValue() int64 {
	return math.MinInt64
}

func (*NewOrder) SideId() uint16 {
	return 2
}

func (*NewOrder) SideSinceVersion() uint16 {
	return 0
}

func (n *NewOrder) SideInActingVersion(actingVersion uint16) bool {
	return actingVersion >= n.SideSinceVersion()
}

func (*NewOrder) SideDeprecated() uint16 {
	return 0
}

func (*NewOrder) SideMetaAttribute(meta int) string {
	switch meta {
	case 1:
		return ""
	case 2:
		return ""
	case 3:
		return ""
	case 4:
		return "required"
	}
	return ""
}

func (*NewOrder) PriceId() uint16 {
	return 3
}

func (*NewOrder) PriceSinceVersion() uint16 {
	return 0
}

func (n *NewOrder) PriceInActingVersion(actingVersion uint16) bool {
	return actingVersion >= n.PriceSinceVersion()
}

func (*NewOrder) PriceDeprecated() uint16 {
	return 0
}

func (*NewOrder) PriceMetaAttribute(meta int) string {
	switch meta {
	case 1:
		return ""
	case 2:
		return ""
	case 3:
		return ""
	case 4:
		return "required"
	}
	return ""
}

func (*NewOrder) PriceMinValue() int64 {
	return math.MinInt64 + 1
}

func (*NewOrder) PriceMaxValue() int64 {
	return math.MaxInt64
}

func (*NewOrder) PriceNullValue() int64 {
	return math.MinInt64
}

func (*NewOrder) QtyId() uint16 {
	return 4
}

func (*NewOrder) QtySinceVersion() uint16 {
	return 0
}

func (n *NewOrder) QtyInActingVersion(actingVersion uint16) bool {
	return actingVersion >= n.QtySinceVersion()
}

func (*NewOrder) QtyDeprecated() uint16 {
	return 0
}

func (*NewOrder) QtyMetaAttribute(meta int) string {
	switch meta {
	case 1:
		return ""
	case 2:
		return ""
	case 3:
		return ""
	case 4:
		return "required"
	}
	return ""
}

func (*NewOrder) QtyMinValue() int64 {
	return math.MinInt64 + 1
}

func (*NewOrder) QtyMaxValue() int64 {
	return math.MaxInt64
}

func (*NewOrder) QtyNullValue() int64 {
	return math.MinInt64
}
