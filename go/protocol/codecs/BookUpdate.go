// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
)

type BookUpdate struct {
	Side         SideEnum
	Price        int64
	AggregateQty int64
	Timestamp    int64
}

func (b *BookUpdate) Encode(_m *SbeGoMarshaller, _w io.Writer, doRangeCheck bool) error {
	if doRangeCheck {
		if err := b.RangeCheck(b.SbeSchemaVersion(), b.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	if err := b.Side.Encode(_m, _w); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, b.Price); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, b.AggregateQty); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, b.Timestamp); err != nil {
		return err
	}
	return nil
}

func (b *BookUpdate) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16, blockLength uint16, doRangeCheck bool) error {
	if b.SideInActingVersion(actingVersion) {
		if err := b.Side.Decode(_m, _r, actingVersion); err != nil {
			return err
		}
	}
	if !b.PriceInActingVersion(actingVersion) {
		b.Price = b.PriceNullValue()
	} else {
		if err := _m.ReadInt64(_r, &b.Price); err != nil {
			return err
		}
	}
	if !b.AggregateQtyInActingVersion(actingVersion) {
		b.AggregateQty = b.AggregateQtyNullValue()
	} else {
		if err := _m.ReadInt64(_r, &b.AggregateQty); err != nil {
			return err
		}
	}
	if !b.TimestampInActingVersion(actingVersion) {
		b.Timestamp = b.TimestampNullValue()
	} else {
		if err := _m.ReadInt64(_r, &b.Timestamp); err != nil {
			return err
		}
	}
	if actingVersion > b.SbeSchemaVersion() && blockLength > b.SbeBlockLength() {
		io.CopyN(ioutil.Discard, _r, int64(blockLength-b.SbeBlockLength()))
	}
	if doRangeCheck {
		if err := b.RangeCheck(actingVersion, b.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	return nil
}

func (b *BookUpdate) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if err := b.Side.RangeCheck(actingVersion, schemaVersion); err != nil {
		return err
	}
	if b.PriceInActingVersion(actingVersion) {
		if b.Price < b.PriceMinValue() || b.Price > b.PriceMaxValue() {
			return fmt.Errorf("Range check failed on b.Price (%v < %v > %v)", b.PriceMinValue(), b.Price, b.PriceMaxValue())
		}
	}
	if b.AggregateQtyInActingVersion(actingVersion) {
		if b.AggregateQty < b.AggregateQtyMinValue() || b.AggregateQty > b.AggregateQtyMaxValue() {
			return fmt.Errorf("Range check failed on b.AggregateQty (%v < %v > %v)", b.AggregateQtyMinValue(), b.AggregateQty, b.AggregateQtyMaxValue())
		}
	}
	if b.TimestampInActingVersion(actingVersion) {
		if b.Timestamp < b.TimestampMinValue() || b.Timestamp > b.TimestampMaxValue() {
			return fmt.Errorf("Range check failed on b.Timestamp (%v < %v > %v)", b.TimestampMinValue(), b.Timestamp, b.TimestampMaxValue())
		}
	}
	return nil
}

func BookUpdateInit(b *BookUpdate) {
	return
}

func (*BookUpdate) SbeBlockLength() (blockLength uint16) {
	return 25
}

func (*BookUpdate) SbeTemplateId() (templateId uint16) {
	return 12
}

func (*BookUpdate) SbeSchemaId() (schemaId uint16) {
	return 901
}

func (*BookUpdate) SbeSchemaVersion() (schemaVersion uint16) {
	return 1
}

func (*BookUpdate) SbeSemanticType() (semanticType []byte) {
	return []byte("")
}

func (*BookUpdate) SbeSemanticVersion() (semanticVersion string) {
	return "1.0"
}

func (*BookUpdate) SideId() uint16 {
	return 1
}

func (*BookUpdate) SideSinceVersion() uint16 {
	return 0
}

func (b *BookUpdate) SideInActingVersion(actingVersion uint16) bool {
	return actingVersion >= b.SideSinceVersion()
}

func (*BookUpdate) SideDeprecated() uint16 {
	return 0
}

func (*BookUpdate) SideMetaAttribute(meta int) string {
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

func (*BookUpdate) PriceId() uint16 {
	return 2
}

func (*BookUpdate) PriceSinceVersion() uint16 {
	return 0
}

func (b *BookUpdate) PriceInActingVersion(actingVersion uint16) bool {
	return actingVersion >= b.PriceSinceVersion()
}

func (*BookUpdate) PriceDeprecated() uint16 {
	return 0
}

func (*BookUpdate) PriceMetaAttribute(meta int) string {
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

func (*BookUpdate) PriceMinValue() int64 {
	return math.MinInt64 + 1
}

func (*BookUpdate) PriceMaxValue() int64 {
	return math.MaxInt64
}

func (*BookUpdate) PriceNullValue() int64 {
	return math.MinInt64
}

func (*BookUpdate) AggregateQtyId() uint16 {
	return 3
}

func (*BookUpdate) AggregateQtySinceVersion() uint16 {
	return 0
}

func (b *BookUpdate) AggregateQtyInActingVersion(actingVersion uint16) bool {
	return actingVersion >= b.AggregateQtySinceVersion()
}

func (*BookUpdate) AggregateQtyDeprecated() uint16 {
	return 0
}

func (*BookUpdate) AggregateQtyMetaAttribute(meta int) string {
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

func (*BookUpdate) AggregateQtyMinValue() int64 {
	return math.MinInt64 + 1
}

func (*BookUpdate) AggregateQtyMaxValue() int64 {
	return math.MaxInt64
}

func (*BookUpdate) AggregateQtyNullValue() int64 {
	return math.MinInt64
}

func (*BookUpdate) TimestampId() uint16 {
	return 4
}

func (*BookUpdate) TimestampSinceVersion() uint16 {
	return 0
}

func (b *BookUpdate) TimestampInActingVersion(actingVersion uint16) bool {
	return actingVersion >= b.TimestampSinceVersion()
}

func (*BookUpdate) TimestampDeprecated() uint16 {
	return 0
}

func (*BookUpdate) TimestampMetaAttribute(meta int) string {
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

func (*BookUpdate) TimestampMinValue() int64 {
	return math.MinInt64 + 1
}

func (*BookUpdate) TimestampMaxValue() int64 {
	return math.MaxInt64
}

func (*BookUpdate) TimestampNullValue() int64 {
	return math.MinInt64
}
