package models

import "time"

type Cliente struct {
	ID                int       `json:"id"`
	Nombre            string    `json:"nombre"`
	Email             string    `json:"email"`
	FechaModificacion time.Time `json:"fecha_modificacion"`
}
