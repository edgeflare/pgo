// Package mqtt provides a real-time MQTT-based interface for PostgreSQL,
// similar to how PostgREST exposes PostgreSQL over HTTP.
package mqtt

// topic: /prefix/OPTIONAL_SCHEMA/TABLE/OPERATION/COL1/VAL1/COL2/VAL2/...
// payload: JSON (object / array)
//
// OPERATION
// c=create, u=update, d=delete, r=read
//
// Example:
// mosquitto_pub -t /example/prefix/public/devices/c -m '{"name":"kitchen-light"}'
//
// mosquitto_sub -t '/example/prefix/public/devices/#' //
//
// mosquitto_pub -t /example/prefix/iot/sensors/r/name/kitchen-light
// mosquitto_pub -t /example/prefix/iot/sensors/read/name/kitchen-light
// mosquitto_pub -t /example/prefix/iot/sensors/u/id/100 -m '{"name":"kitchen-light", "status": 0}'
// mosquitto_pub -t /example/prefix/iot/sensors/d/id/100

// In all cases, a response is published, unless disabled, to the /response/original/topic
// mosquitto_sub -t /response/example/prefix/iot/sensors/d/id/100
//
// With a trailing /batch in topic, it's possible to supply an array of json objects for supported operations
// mosquitto_pub -t /example/prefix/devices/c/batch -m '[{"name":"device1"}, {"name":"device2"}]'
