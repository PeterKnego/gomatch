// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
)

type ExecutionReport struct {
	OrderId       int64
	ClientOrderId int64
	Status        OrderStatusEnum
	Reason        RejectReasonEnum
	Side          SideEnum
	Price         int64
	Qty           int64
	RemainingQty  int64
	Timestamp     int64
}

func (e *ExecutionReport) Encode(_m *SbeGoMarshaller, _w io.Writer, doRangeCheck bool) error {
	if doRangeCheck {
		if err := e.RangeCheck(e.SbeSchemaVersion(), e.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	if err := _m.WriteInt64(_w, e.OrderId); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, e.ClientOrderId); err != nil {
		return err
	}
	if err := e.Status.Encode(_m, _w); err != nil {
		return err
	}
	if err := e.Reason.Encode(_m, _w); err != nil {
		return err
	}
	if err := e.Side.Encode(_m, _w); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, e.Price); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, e.Qty); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, e.RemainingQty); err != nil {
		return err
	}
	if err := _m.WriteInt64(_w, e.Timestamp); err != nil {
		return err
	}
	return nil
}

func (e *ExecutionReport) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16, blockLength uint16, doRangeCheck bool) error {
	if !e.OrderIdInActingVersion(actingVersion) {
		e.OrderId = e.OrderIdNullValue()
	} else {
		if err := _m.ReadInt64(_r, &e.OrderId); err != nil {
			return err
		}
	}
	if !e.ClientOrderIdInActingVersion(actingVersion) {
		e.ClientOrderId = e.ClientOrderIdNullValue()
	} else {
		if err := _m.ReadInt64(_r, &e.ClientOrderId); err != nil {
			return err
		}
	}
	if e.StatusInActingVersion(actingVersion) {
		if err := e.Status.Decode(_m, _r, actingVersion); err != nil {
			return err
		}
	}
	if e.ReasonInActingVersion(actingVersion) {
		if err := e.Reason.Decode(_m, _r, actingVersion); err != nil {
			return err
		}
	}
	if e.SideInActingVersion(actingVersion) {
		if err := e.Side.Decode(_m, _r, actingVersion); err != nil {
			return err
		}
	}
	if !e.PriceInActingVersion(actingVersion) {
		e.Price = e.PriceNullValue()
	} else {
		if err := _m.ReadInt64(_r, &e.Price); err != nil {
			return err
		}
	}
	if !e.QtyInActingVersion(actingVersion) {
		e.Qty = e.QtyNullValue()
	} else {
		if err := _m.ReadInt64(_r, &e.Qty); err != nil {
			return err
		}
	}
	if !e.RemainingQtyInActingVersion(actingVersion) {
		e.RemainingQty = e.RemainingQtyNullValue()
	} else {
		if err := _m.ReadInt64(_r, &e.RemainingQty); err != nil {
			return err
		}
	}
	if !e.TimestampInActingVersion(actingVersion) {
		e.Timestamp = e.TimestampNullValue()
	} else {
		if err := _m.ReadInt64(_r, &e.Timestamp); err != nil {
			return err
		}
	}
	if actingVersion > e.SbeSchemaVersion() && blockLength > e.SbeBlockLength() {
		io.CopyN(ioutil.Discard, _r, int64(blockLength-e.SbeBlockLength()))
	}
	if doRangeCheck {
		if err := e.RangeCheck(actingVersion, e.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	return nil
}

func (e *ExecutionReport) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if e.OrderIdInActingVersion(actingVersion) {
		if e.OrderId < e.OrderIdMinValue() || e.OrderId > e.OrderIdMaxValue() {
			return fmt.Errorf("Range check failed on e.OrderId (%v < %v > %v)", e.OrderIdMinValue(), e.OrderId, e.OrderIdMaxValue())
		}
	}
	if e.ClientOrderIdInActingVersion(actingVersion) {
		if e.ClientOrderId < e.ClientOrderIdMinValue() || e.ClientOrderId > e.ClientOrderIdMaxValue() {
			return fmt.Errorf("Range check failed on e.ClientOrderId (%v < %v > %v)", e.ClientOrderIdMinValue(), e.ClientOrderId, e.ClientOrderIdMaxValue())
		}
	}
	if err := e.Status.RangeCheck(actingVersion, schemaVersion); err != nil {
		return err
	}
	if err := e.Reason.RangeCheck(actingVersion, schemaVersion); err != nil {
		return err
	}
	if err := e.Side.RangeCheck(actingVersion, schemaVersion); err != nil {
		return err
	}
	if e.PriceInActingVersion(actingVersion) {
		if e.Price < e.PriceMinValue() || e.Price > e.PriceMaxValue() {
			return fmt.Errorf("Range check failed on e.Price (%v < %v > %v)", e.PriceMinValue(), e.Price, e.PriceMaxValue())
		}
	}
	if e.QtyInActingVersion(actingVersion) {
		if e.Qty < e.QtyMinValue() || e.Qty > e.QtyMaxValue() {
			return fmt.Errorf("Range check failed on e.Qty (%v < %v > %v)", e.QtyMinValue(), e.Qty, e.QtyMaxValue())
		}
	}
	if e.RemainingQtyInActingVersion(actingVersion) {
		if e.RemainingQty < e.RemainingQtyMinValue() || e.RemainingQty > e.RemainingQtyMaxValue() {
			return fmt.Errorf("Range check failed on e.RemainingQty (%v < %v > %v)", e.RemainingQtyMinValue(), e.RemainingQty, e.RemainingQtyMaxValue())
		}
	}
	if e.TimestampInActingVersion(actingVersion) {
		if e.Timestamp < e.TimestampMinValue() || e.Timestamp > e.TimestampMaxValue() {
			return fmt.Errorf("Range check failed on e.Timestamp (%v < %v > %v)", e.TimestampMinValue(), e.Timestamp, e.TimestampMaxValue())
		}
	}
	return nil
}

func ExecutionReportInit(e *ExecutionReport) {
	return
}

func (*ExecutionReport) SbeBlockLength() (blockLength uint16) {
	return 51
}

func (*ExecutionReport) SbeTemplateId() (templateId uint16) {
	return 10
}

func (*ExecutionReport) SbeSchemaId() (schemaId uint16) {
	return 901
}

func (*ExecutionReport) SbeSchemaVersion() (schemaVersion uint16) {
	return 1
}

func (*ExecutionReport) SbeSemanticType() (semanticType []byte) {
	return []byte("")
}

func (*ExecutionReport) SbeSemanticVersion() (semanticVersion string) {
	return "1.0"
}

func (*ExecutionReport) OrderIdId() uint16 {
	return 1
}

func (*ExecutionReport) OrderIdSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) OrderIdInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.OrderIdSinceVersion()
}

func (*ExecutionReport) OrderIdDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) OrderIdMetaAttribute(meta int) string {
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

func (*ExecutionReport) OrderIdMinValue() int64 {
	return math.MinInt64 + 1
}

func (*ExecutionReport) OrderIdMaxValue() int64 {
	return math.MaxInt64
}

func (*ExecutionReport) OrderIdNullValue() int64 {
	return math.MinInt64
}

func (*ExecutionReport) ClientOrderIdId() uint16 {
	return 2
}

func (*ExecutionReport) ClientOrderIdSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) ClientOrderIdInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.ClientOrderIdSinceVersion()
}

func (*ExecutionReport) ClientOrderIdDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) ClientOrderIdMetaAttribute(meta int) string {
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

func (*ExecutionReport) ClientOrderIdMinValue() int64 {
	return math.MinInt64 + 1
}

func (*ExecutionReport) ClientOrderIdMaxValue() int64 {
	return math.MaxInt64
}

func (*ExecutionReport) ClientOrderIdNullValue() int64 {
	return math.MinInt64
}

func (*ExecutionReport) StatusId() uint16 {
	return 3
}

func (*ExecutionReport) StatusSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) StatusInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.StatusSinceVersion()
}

func (*ExecutionReport) StatusDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) StatusMetaAttribute(meta int) string {
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

func (*ExecutionReport) ReasonId() uint16 {
	return 4
}

func (*ExecutionReport) ReasonSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) ReasonInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.ReasonSinceVersion()
}

func (*ExecutionReport) ReasonDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) ReasonMetaAttribute(meta int) string {
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

func (*ExecutionReport) SideId() uint16 {
	return 5
}

func (*ExecutionReport) SideSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) SideInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.SideSinceVersion()
}

func (*ExecutionReport) SideDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) SideMetaAttribute(meta int) string {
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

func (*ExecutionReport) PriceId() uint16 {
	return 6
}

func (*ExecutionReport) PriceSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) PriceInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.PriceSinceVersion()
}

func (*ExecutionReport) PriceDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) PriceMetaAttribute(meta int) string {
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

func (*ExecutionReport) PriceMinValue() int64 {
	return math.MinInt64 + 1
}

func (*ExecutionReport) PriceMaxValue() int64 {
	return math.MaxInt64
}

func (*ExecutionReport) PriceNullValue() int64 {
	return math.MinInt64
}

func (*ExecutionReport) QtyId() uint16 {
	return 7
}

func (*ExecutionReport) QtySinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) QtyInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.QtySinceVersion()
}

func (*ExecutionReport) QtyDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) QtyMetaAttribute(meta int) string {
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

func (*ExecutionReport) QtyMinValue() int64 {
	return math.MinInt64 + 1
}

func (*ExecutionReport) QtyMaxValue() int64 {
	return math.MaxInt64
}

func (*ExecutionReport) QtyNullValue() int64 {
	return math.MinInt64
}

func (*ExecutionReport) RemainingQtyId() uint16 {
	return 8
}

func (*ExecutionReport) RemainingQtySinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) RemainingQtyInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.RemainingQtySinceVersion()
}

func (*ExecutionReport) RemainingQtyDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) RemainingQtyMetaAttribute(meta int) string {
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

func (*ExecutionReport) RemainingQtyMinValue() int64 {
	return math.MinInt64 + 1
}

func (*ExecutionReport) RemainingQtyMaxValue() int64 {
	return math.MaxInt64
}

func (*ExecutionReport) RemainingQtyNullValue() int64 {
	return math.MinInt64
}

func (*ExecutionReport) TimestampId() uint16 {
	return 9
}

func (*ExecutionReport) TimestampSinceVersion() uint16 {
	return 0
}

func (e *ExecutionReport) TimestampInActingVersion(actingVersion uint16) bool {
	return actingVersion >= e.TimestampSinceVersion()
}

func (*ExecutionReport) TimestampDeprecated() uint16 {
	return 0
}

func (*ExecutionReport) TimestampMetaAttribute(meta int) string {
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

func (*ExecutionReport) TimestampMinValue() int64 {
	return math.MinInt64 + 1
}

func (*ExecutionReport) TimestampMaxValue() int64 {
	return math.MaxInt64
}

func (*ExecutionReport) TimestampNullValue() int64 {
	return math.MinInt64
}
