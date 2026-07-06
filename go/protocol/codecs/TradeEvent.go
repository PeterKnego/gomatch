// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
)

type TradeEvent struct {
	Price        int64
	Qty          int64
	MakerOrderId int64
	TakerOrderId int64
	Timestamp    int64
}

func (t *TradeEvent) Encode(_m *SbeGoMarshaller, _w io.Writer, doRangeCheck bool) error {
	if doRangeCheck {
		if err := t.RangeCheck(t.SbeSchemaVersion(), t.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	if err := _m.WriteInt64(_w, t.Price); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, t.Qty); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, t.MakerOrderId); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, t.TakerOrderId); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, t.Timestamp); err != nil {
		return err
	}
	return nil
}

func (t *TradeEvent) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16, blockLength uint16, doRangeCheck bool) error {
	if !t.PriceInActingVersion(actingVersion) {
		t.Price = t.PriceNullValue()
	} else {
		if err := _m.ReadInt64(_r, &t.Price); err != nil {
			return err
		}
	}
	if !t.QtyInActingVersion(actingVersion) {
		t.Qty = t.QtyNullValue()
	} else {
		if err := _m.ReadInt64(_r, &t.Qty); err != nil {
			return err
		}
	}
	if !t.MakerOrderIdInActingVersion(actingVersion) {
		t.MakerOrderId = t.MakerOrderIdNullValue()
	} else {
		if err := _m.ReadInt64(_r, &t.MakerOrderId); err != nil {
			return err
		}
	}
	if !t.TakerOrderIdInActingVersion(actingVersion) {
		t.TakerOrderId = t.TakerOrderIdNullValue()
	} else {
		if err := _m.ReadInt64(_r, &t.TakerOrderId); err != nil {
			return err
		}
	}
	if !t.TimestampInActingVersion(actingVersion) {
		t.Timestamp = t.TimestampNullValue()
	} else {
		if err := _m.ReadInt64(_r, &t.Timestamp); err != nil {
			return err
		}
	}
	if actingVersion > t.SbeSchemaVersion() && blockLength > t.SbeBlockLength() {
		io.CopyN(ioutil.Discard, _r, int64(blockLength-t.SbeBlockLength()))
	}
	if doRangeCheck {
		if err := t.RangeCheck(actingVersion, t.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	return nil
}

func (t *TradeEvent) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if t.PriceInActingVersion(actingVersion) {
		if t.Price < t.PriceMinValue() || t.Price > t.PriceMaxValue() {
			return fmt.Errorf("Range check failed on t.Price (%v < %v > %v)", t.PriceMinValue(), t.Price, t.PriceMaxValue())
		}
	}
	if t.QtyInActingVersion(actingVersion) {
		if t.Qty < t.QtyMinValue() || t.Qty > t.QtyMaxValue() {
			return fmt.Errorf("Range check failed on t.Qty (%v < %v > %v)", t.QtyMinValue(), t.Qty, t.QtyMaxValue())
		}
	}
	if t.MakerOrderIdInActingVersion(actingVersion) {
		if t.MakerOrderId < t.MakerOrderIdMinValue() || t.MakerOrderId > t.MakerOrderIdMaxValue() {
			return fmt.Errorf("Range check failed on t.MakerOrderId (%v < %v > %v)", t.MakerOrderIdMinValue(), t.MakerOrderId, t.MakerOrderIdMaxValue())
		}
	}
	if t.TakerOrderIdInActingVersion(actingVersion) {
		if t.TakerOrderId < t.TakerOrderIdMinValue() || t.TakerOrderId > t.TakerOrderIdMaxValue() {
			return fmt.Errorf("Range check failed on t.TakerOrderId (%v < %v > %v)", t.TakerOrderIdMinValue(), t.TakerOrderId, t.TakerOrderIdMaxValue())
		}
	}
	if t.TimestampInActingVersion(actingVersion) {
		if t.Timestamp < t.TimestampMinValue() || t.Timestamp > t.TimestampMaxValue() {
			return fmt.Errorf("Range check failed on t.Timestamp (%v < %v > %v)", t.TimestampMinValue(), t.Timestamp, t.TimestampMaxValue())
		}
	}
	return nil
}

func TradeEventInit(t *TradeEvent) {
	return
}

func (*TradeEvent) SbeBlockLength() (blockLength uint16) {
	return 40
}

func (*TradeEvent) SbeTemplateId() (templateId uint16) {
	return 11
}

func (*TradeEvent) SbeSchemaId() (schemaId uint16) {
	return 901
}

func (*TradeEvent) SbeSchemaVersion() (schemaVersion uint16) {
	return 1
}

func (*TradeEvent) SbeSemanticType() (semanticType []byte) {
	return []byte("")
}

func (*TradeEvent) SbeSemanticVersion() (semanticVersion string) {
	return "1.0"
}

func (*TradeEvent) PriceId() uint16 {
	return 1
}

func (*TradeEvent) PriceSinceVersion() uint16 {
	return 0
}

func (t *TradeEvent) PriceInActingVersion(actingVersion uint16) bool {
	return actingVersion >= t.PriceSinceVersion()
}

func (*TradeEvent) PriceDeprecated() uint16 {
	return 0
}

func (*TradeEvent) PriceMetaAttribute(meta int) string {
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

func (*TradeEvent) PriceMinValue() int64 {
	return math.MinInt64 + 1
}

func (*TradeEvent) PriceMaxValue() int64 {
	return math.MaxInt64
}

func (*TradeEvent) PriceNullValue() int64 {
	return math.MinInt64
}

func (*TradeEvent) QtyId() uint16 {
	return 2
}

func (*TradeEvent) QtySinceVersion() uint16 {
	return 0
}

func (t *TradeEvent) QtyInActingVersion(actingVersion uint16) bool {
	return actingVersion >= t.QtySinceVersion()
}

func (*TradeEvent) QtyDeprecated() uint16 {
	return 0
}

func (*TradeEvent) QtyMetaAttribute(meta int) string {
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

func (*TradeEvent) QtyMinValue() int64 {
	return math.MinInt64 + 1
}

func (*TradeEvent) QtyMaxValue() int64 {
	return math.MaxInt64
}

func (*TradeEvent) QtyNullValue() int64 {
	return math.MinInt64
}

func (*TradeEvent) MakerOrderIdId() uint16 {
	return 3
}

func (*TradeEvent) MakerOrderIdSinceVersion() uint16 {
	return 0
}

func (t *TradeEvent) MakerOrderIdInActingVersion(actingVersion uint16) bool {
	return actingVersion >= t.MakerOrderIdSinceVersion()
}

func (*TradeEvent) MakerOrderIdDeprecated() uint16 {
	return 0
}

func (*TradeEvent) MakerOrderIdMetaAttribute(meta int) string {
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

func (*TradeEvent) MakerOrderIdMinValue() int64 {
	return math.MinInt64 + 1
}

func (*TradeEvent) MakerOrderIdMaxValue() int64 {
	return math.MaxInt64
}

func (*TradeEvent) MakerOrderIdNullValue() int64 {
	return math.MinInt64
}

func (*TradeEvent) TakerOrderIdId() uint16 {
	return 4
}

func (*TradeEvent) TakerOrderIdSinceVersion() uint16 {
	return 0
}

func (t *TradeEvent) TakerOrderIdInActingVersion(actingVersion uint16) bool {
	return actingVersion >= t.TakerOrderIdSinceVersion()
}

func (*TradeEvent) TakerOrderIdDeprecated() uint16 {
	return 0
}

func (*TradeEvent) TakerOrderIdMetaAttribute(meta int) string {
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

func (*TradeEvent) TakerOrderIdMinValue() int64 {
	return math.MinInt64 + 1
}

func (*TradeEvent) TakerOrderIdMaxValue() int64 {
	return math.MaxInt64
}

func (*TradeEvent) TakerOrderIdNullValue() int64 {
	return math.MinInt64
}

func (*TradeEvent) TimestampId() uint16 {
	return 5
}

func (*TradeEvent) TimestampSinceVersion() uint16 {
	return 0
}

func (t *TradeEvent) TimestampInActingVersion(actingVersion uint16) bool {
	return actingVersion >= t.TimestampSinceVersion()
}

func (*TradeEvent) TimestampDeprecated() uint16 {
	return 0
}

func (*TradeEvent) TimestampMetaAttribute(meta int) string {
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

func (*TradeEvent) TimestampMinValue() int64 {
	return math.MinInt64 + 1
}

func (*TradeEvent) TimestampMaxValue() int64 {
	return math.MaxInt64
}

func (*TradeEvent) TimestampNullValue() int64 {
	return math.MinInt64
}
