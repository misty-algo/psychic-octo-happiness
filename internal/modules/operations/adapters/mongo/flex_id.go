package mongo

import "go.mongodb.org/mongo-driver/v2/bson"

// tickets.orderId 存在两种历史形态：旧工单存 ObjectId，
// 客服域落地后按业务单号（HXZ202610300001）存字符串。
// 固定 bson.ObjectID 解码遇到字符串会让整条文档解码失败、列表接口 500，
// 因此统一用 bson.RawValue（驱动内建支持）保留原始类型，
// 取值时按实际类型转成展示文本，写回时 RawValue 原样还原、无损。

// flexIDString 取关联单号的展示文本：ObjectId 取 hex，字符串原样返回。
func flexIDString(rv bson.RawValue) string {
	switch rv.Type {
	case bson.TypeObjectID:
		return rv.ObjectID().Hex()
	case bson.TypeString:
		return rv.StringValue()
	default:
		return ""
	}
}
