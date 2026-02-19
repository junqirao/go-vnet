package protocol

import (
	"io"

	"go-vnet/common/metrics"
)

type (
	metricsWrapper struct {
		rwc io.ReadWriteCloser
		m   *metrics.TransportMetrics
	}
)

func NewMetricsWrapper(rwc io.ReadWriteCloser, m *metrics.TransportMetrics) io.ReadWriteCloser {
	return &metricsWrapper{
		rwc: rwc,
		m:   m,
	}
}

func (m metricsWrapper) Read(p []byte) (n int, err error) {
	n, err = m.rwc.Read(p)
	m.m.RxBytes.Add(uint64(n))
	m.m.RxPackets.Add(1)
	return
}

func (m metricsWrapper) Write(p []byte) (n int, err error) {
	n, err = m.rwc.Write(p)
	m.m.TxBytes.Add(uint64(n))
	m.m.TxPackets.Add(1)
	return
}

func (m metricsWrapper) Close() error {
	return m.rwc.Close()
}
