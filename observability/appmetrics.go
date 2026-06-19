package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Helper tiện tạo metric nghiệp vụ dùng global MeterProvider (đã set bởi New;
// no-op khi observability tắt). Tên scope thường là module path của service.
//
//	m := observability.Meter("orders")
//	ordersCreated := observability.MustInt64Counter(m, "orders.created", "số đơn tạo", "{order}")
//	ordersCreated.Add(ctx, 1)

// Meter trả Meter cho instrumentation scope name.
func Meter(name string) metric.Meter { return otel.Meter(name) }

// Int64Counter tạo counter Int64 (đếm tăng dần).
func Int64Counter(m metric.Meter, name, desc, unit string) (metric.Int64Counter, error) {
	return m.Int64Counter(name, metric.WithDescription(desc), metric.WithUnit(unit))
}

// MustInt64Counter như Int64Counter nhưng panic nếu lỗi (dùng lúc khởi tạo).
func MustInt64Counter(m metric.Meter, name, desc, unit string) metric.Int64Counter {
	c, err := Int64Counter(m, name, desc, unit)
	if err != nil {
		panic("observability: create counter " + name + ": " + err.Error())
	}
	return c
}

// Float64Histogram tạo histogram Float64 (phân phối giá trị, vd latency).
func Float64Histogram(m metric.Meter, name, desc, unit string) (metric.Float64Histogram, error) {
	return m.Float64Histogram(name, metric.WithDescription(desc), metric.WithUnit(unit))
}

// MustFloat64Histogram như Float64Histogram nhưng panic nếu lỗi.
func MustFloat64Histogram(m metric.Meter, name, desc, unit string) metric.Float64Histogram {
	h, err := Float64Histogram(m, name, desc, unit)
	if err != nil {
		panic("observability: create histogram " + name + ": " + err.Error())
	}
	return h
}

// Int64UpDownCounter tạo counter tăng/giảm (vd số kết nối đang mở).
func Int64UpDownCounter(m metric.Meter, name, desc, unit string) (metric.Int64UpDownCounter, error) {
	return m.Int64UpDownCounter(name, metric.WithDescription(desc), metric.WithUnit(unit))
}
