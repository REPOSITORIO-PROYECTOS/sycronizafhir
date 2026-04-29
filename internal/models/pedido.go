package models

import "time"

type Pedido struct {
	IDPedidoNube  string      `json:"id"`
	IDCliente     int         `json:"id_cliente"`
	Total         float64     `json:"total"`
	FechaCreacion time.Time   `json:"fecha_creacion"`
	Detalle       interface{} `json:"detalle"`
}
