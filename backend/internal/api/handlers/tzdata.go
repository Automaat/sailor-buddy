package handlers

// The sign-up deadline is a Europe/Warsaw calendar date (see clubZone); embed
// the zone database so the rule holds on images without system tzdata.
import _ "time/tzdata"
