// Generated SBE (Simple Binary Encoding) message codec

package codecs

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
)

type CancelOrder struct {
	OrderId int64
}

func (c *CancelOrder) Encode(_m *SbeGoMarshaller, _w io.Writer, doRangeCheck bool) error {
	if doRangeCheck {
		if err := c.RangeCheck(c.SbeSchemaVersion(), c.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	if err := _m.WriteInt64(_w, c.OrderId); err != nil {
		return err
	}
	return nil
}

func (c *CancelOrder) Decode(_m *SbeGoMarshaller, _r io.Reader, actingVersion uint16, blockLength uint16, doRangeCheck bool) error {
	if !c.OrderIdInActingVersion(actingVersion) {
		c.OrderId = c.OrderIdNullValue()
	} else {
		if err := _m.ReadInt64(_r, &c.OrderId); err != nil {
			return err
		}
	}
	if actingVersion > c.SbeSchemaVersion() && blockLength > c.SbeBlockLength() {
		io.CopyN(ioutil.Discard, _r, int64(blockLength-c.SbeBlockLength()))
	}
	if doRangeCheck {
		if err := c.RangeCheck(actingVersion, c.SbeSchemaVersion()); err != nil {
			return err
		}
	}
	return nil
}

func (c *CancelOrder) RangeCheck(actingVersion uint16, schemaVersion uint16) error {
	if c.OrderIdInActingVersion(actingVersion) {
		if c.OrderId < c.OrderIdMinValue() || c.OrderId > c.OrderIdMaxValue() {
			return fmt.Errorf("Range check failed on c.OrderId (%v < %v > %v)", c.OrderIdMinValue(), c.OrderId, c.OrderIdMaxValue())
		}
	}
	return nil
}

func CancelOrderInit(c *CancelOrder) {
	return
}

func (*CancelOrder) SbeBlockLength() (blockLength uint16) {
	return 8
}

func (*CancelOrder) SbeTemplateId() (templateId uint16) {
	return 2
}

func (*CancelOrder) SbeSchemaId() (schemaId uint16) {
	return 901
}

func (*CancelOrder) SbeSchemaVersion() (schemaVersion uint16) {
	return 1
}

func (*CancelOrder) SbeSemanticType() (semanticType []byte) {
	return []byte("")
}

func (*CancelOrder) SbeSemanticVersion() (semanticVersion string) {
	return "1.0"
}

func (*CancelOrder) OrderIdId() uint16 {
	return 1
}

func (*CancelOrder) OrderIdSinceVersion() uint16 {
	return 0
}

func (c *CancelOrder) OrderIdInActingVersion(actingVersion uint16) bool {
	return actingVersion >= c.OrderIdSinceVersion()
}

func (*CancelOrder) OrderIdDeprecated() uint16 {
	return 0
}

func (*CancelOrder) OrderIdMetaAttribute(meta int) string {
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

func (*CancelOrder) OrderIdMinValue() int64 {
	return math.MinInt64 + 1
}

func (*CancelOrder) OrderIdMaxValue() int64 {
	return math.MaxInt64
}

func (*CancelOrder) OrderIdNullValue() int64 {
	return math.MinInt64
}
