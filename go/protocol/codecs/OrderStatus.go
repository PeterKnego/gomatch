// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"reflect"
)

type OrderStatusEnum int8
type OrderStatusValues struct {
	ACCEPTED         OrderStatusEnum
	REJECTED         OrderStatusEnum
	PARTIALLY_FILLED OrderStatusEnum
	FILLED           OrderStatusEnum
	CANCELED         OrderStatusEnum
	NullValue        OrderStatusEnum
}

var OrderStatus = OrderStatusValues{0, 1, 2, 3, 4, -128}

func (o OrderStatusEnum) Encode(_m *SbeGoMarshaller, _w io.Writer) error {
	if err := _m.WriteInt8(_w, int8(o)); err != nil {
		return err
	}
	return nil
}

func (o *OrderStatusEnum) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16) error {
	if err := _m.ReadInt8(_r, (*int8)(o)); err != nil {
		return err
	}
	return nil
}

func (o OrderStatusEnum) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if actingVersion > schemaVersion {
		return nil
	}
	value := reflect.ValueOf(OrderStatus)
	for idx := 0; idx < value.NumField(); idx++ {
		if o == value.Field(idx).Interface() {
			return nil
		}
	}
	return fmt.Errorf("Range check failed on OrderStatus, unknown enumeration value %d", o)
}

func (*OrderStatusEnum) EncodedLength() int64 {
	return 1
}

func (*OrderStatusEnum) ACCEPTEDSinceVersion() uint16 {
	return 0
}

func (o *OrderStatusEnum) ACCEPTEDInActingVersion(actingVersion uint16) bool {
	return actingVersion >= o.ACCEPTEDSinceVersion()
}

func (*OrderStatusEnum) ACCEPTEDDeprecated() uint16 {
	return 0
}

func (*OrderStatusEnum) REJECTEDSinceVersion() uint16 {
	return 0
}

func (o *OrderStatusEnum) REJECTEDInActingVersion(actingVersion uint16) bool {
	return actingVersion >= o.REJECTEDSinceVersion()
}

func (*OrderStatusEnum) REJECTEDDeprecated() uint16 {
	return 0
}

func (*OrderStatusEnum) PARTIALLY_FILLEDSinceVersion() uint16 {
	return 0
}

func (o *OrderStatusEnum) PARTIALLY_FILLEDInActingVersion(actingVersion uint16) bool {
	return actingVersion >= o.PARTIALLY_FILLEDSinceVersion()
}

func (*OrderStatusEnum) PARTIALLY_FILLEDDeprecated() uint16 {
	return 0
}

func (*OrderStatusEnum) FILLEDSinceVersion() uint16 {
	return 0
}

func (o *OrderStatusEnum) FILLEDInActingVersion(actingVersion uint16) bool {
	return actingVersion >= o.FILLEDSinceVersion()
}

func (*OrderStatusEnum) FILLEDDeprecated() uint16 {
	return 0
}

func (*OrderStatusEnum) CANCELEDSinceVersion() uint16 {
	return 0
}

func (o *OrderStatusEnum) CANCELEDInActingVersion(actingVersion uint16) bool {
	return actingVersion >= o.CANCELEDSinceVersion()
}

func (*OrderStatusEnum) CANCELEDDeprecated() uint16 {
	return 0
}
