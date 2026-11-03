package db

import "time"

// ERPLocation es la zona en la que Gestiona/Mica y la tienda graban
// fecha_modificacion / hora_modificacion (hora de Argentina, sin horario de verano).
// Se usa una zona fija para no depender de tzdata en Windows.
var ERPLocation = time.FixedZone("ART", -3*60*60)

// ERPWallClock devuelve el instante t expresado como "hora de pared" de Argentina,
// sin zona (marcado UTC), que es como quedan guardadas las columnas
// fecha_modificacion/hora_modificacion (timestamp/date sin zona).
func ERPWallClock(t time.Time) time.Time {
	local := t.In(ERPLocation)
	return time.Date(local.Year(), local.Month(), local.Day(),
		local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
}
