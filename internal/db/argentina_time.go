package db

import (
	"time"
	_ "time/tzdata" // Windows (Mica) puede no tener base de zonas horarias.
)

// argentinaLocation: la tienda y Gestiona escriben fecha/hora_modificacion en
// hora local de Argentina (UTC-3, sin horario de verano).
var argentinaLocation = loadArgentinaLocation()

func loadArgentinaLocation() *time.Location {
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		return time.FixedZone("ART", -3*60*60)
	}
	return loc
}

// ArgentinaLocation devuelve la zona usada para columnas de modificación.
func ArgentinaLocation() *time.Location { return argentinaLocation }

// ToArgentinaWallClock convierte un instante real al "reloj de pared" argentino
// etiquetado como UTC, que es como RowModifiedAt arma fecha+hora y como
// Postgres compara columnas timestamp/date sin zona.
func ToArgentinaWallClock(t time.Time) time.Time {
	local := t.In(argentinaLocation)
	return time.Date(local.Year(), local.Month(), local.Day(),
		local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
}
