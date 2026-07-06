// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"reflect"
)

type RejectReasonEnum int8
type RejectReasonValues struct {
	NONE          RejectReasonEnum
	BAD_QTY       RejectReasonEnum
	BAD_PRICE     RejectReasonEnum
	UNKNOWN_ORDER RejectReasonEnum
	NOT_OWNER     RejectReasonEnum
	NullValue     RejectReasonEnum
}

var RejectReason = RejectReasonValues{0, 1, 2, 3, 4, -128}

func (r RejectReasonEnum) Encode(_m *SbeGoMarshaller, _w io.Writer) error {
	if err := _m.WriteInt8(_w, int8(r)); err != nil {
		return err
	}
	return nil
}

func (r *RejectReasonEnum) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16) error {
	if err := _m.ReadInt8(_r, (*int8)(r)); err != nil {
		return err
	}
	return nil
}

func (r RejectReasonEnum) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if actingVersion > schemaVersion {
		return nil
	}
	value := reflect.ValueOf(RejectReason)
	for idx := 0; idx < value.NumField(); idx++ {
		if r == value.Field(idx).Interface() {
			return nil
		}
	}
	return fmt.Errorf("Range check failed on RejectReason, unknown enumeration value %d", r)
}

func (*RejectReasonEnum) EncodedLength() int64 {
	return 1
}

func (*RejectReasonEnum) NONESinceVersion() uint16 {
	return 0
}

func (r *RejectReasonEnum) NONEInActingVersion(actingVersion uint16) bool {
	return actingVersion >= r.NONESinceVersion()
}

func (*RejectReasonEnum) NONEDeprecated() uint16 {
	return 0
}

func (*RejectReasonEnum) BAD_QTYSinceVersion() uint16 {
	return 0
}

func (r *RejectReasonEnum) BAD_QTYInActingVersion(actingVersion uint16) bool {
	return actingVersion >= r.BAD_QTYSinceVersion()
}

func (*RejectReasonEnum) BAD_QTYDeprecated() uint16 {
	return 0
}

func (*RejectReasonEnum) BAD_PRICESinceVersion() uint16 {
	return 0
}

func (r *RejectReasonEnum) BAD_PRICEInActingVersion(actingVersion uint16) bool {
	return actingVersion >= r.BAD_PRICESinceVersion()
}

func (*RejectReasonEnum) BAD_PRICEDeprecated() uint16 {
	return 0
}

func (*RejectReasonEnum) UNKNOWN_ORDERSinceVersion() uint16 {
	return 0
}

func (r *RejectReasonEnum) UNKNOWN_ORDERInActingVersion(actingVersion uint16) bool {
	return actingVersion >= r.UNKNOWN_ORDERSinceVersion()
}

func (*RejectReasonEnum) UNKNOWN_ORDERDeprecated() uint16 {
	return 0
}

func (*RejectReasonEnum) NOT_OWNERSinceVersion() uint16 {
	return 0
}

func (r *RejectReasonEnum) NOT_OWNERInActingVersion(actingVersion uint16) bool {
	return actingVersion >= r.NOT_OWNERSinceVersion()
}

func (*RejectReasonEnum) NOT_OWNERDeprecated() uint16 {
	return 0
}
