# cbyge

For this project, I reverse engineered the "C by GE" app for controlling GE smart lightbulbs. I have a few WiFi-connected smart bulbs, and I wanted more fine-grained control over them (e.g. with my Fitbit, automated programs, etc.). To do this, I started by decompiling the Android app, and then reverse engineered the binary protocol that the app uses to talk to a server.

The final product of this project is a high-level Go API for enumerating lightbulbs, getting their status, and changing their properties (e.g. brightness and color tone).

# Usage

To create a session, simply do:

```go
session, err := cbyge.NewControllerLogin("my_email", "my_password")
// Handle error...
```

Once you have a session, you can enumerate devices like so:

```go
devs, err := session.Devices()
// Handle error...
for _, x := range devs {
    fmt.Println(x.Name())
}
```

You can control bulbs like so:

```go
x := devs[0]
session.SetDeviceStatus(x, true) // turn on
session.SetDeviceLum(x, 50)      // set brightness
session.SetDeviceCT(x, 100)      // set color tone (100=blue, 0=orange)
```

You can also query a bulb's current settings:

```go
status, err := session.DeviceStatus(x)
// Handle error...
fmt.Println(status.IsOn)
fmt.Println(status.ColorTone)
```
