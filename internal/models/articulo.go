package models

import "time"

type Articulo struct {
	ID                int       `json:"id"`
	Nombre            string    `json:"nombre"`
	Precio            float64   `json:"precio"`
	Stock             int       `json:"stock"`
	FechaModificacion time.Time `json:"fecha_modificacion"`
}
