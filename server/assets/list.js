(function () {

    const TONE_0 = [255, 196, 0];
    const TONE_100 = [166, 234, 245];

    class DeviceList {
        constructor() {
            this.element = document.getElementById('devices');
            this.devices = [];
        }

        update(devices) {
            this.devices = [];
            this.element.innerHTML = '';

            devices.forEach((info) => {
                const device = new Device(info);
                this.element.appendChild(device.element);
                this.devices.push(device);
                device.fetchUpdate();
            });
        }

        showError(err) {
            this.element.innerHTML = '';
            const errorElem = makeElem('div', 'devices-error', { textContent: err });
            this.element.appendChild(errorElem);
        }
    }

    class Device {
        constructor(info) {
            this.info = info;
            this.status = null;

            this.element = makeElem('div', 'device');

            this.name = makeElem('label', 'device-name', { textContent: info.name });
            this.element.appendChild(this.name);

            this.onOff = makeElem('div', 'device-on-off');
            this.element.appendChild(this.onOff);
            this.onOff.addEventListener('click', () => this.toggleOnOff());

            this.colorControls = makeElem('div', 'device-color-controls');
            this.element.appendChild(this.colorControls);

            this.brightnessButton = makeElem(
                'button',
                'brightness-button device-color-controls-button',
            );
            this.brightnessButton.addEventListener('click', () => this.editBrightness());
            this.colorControls.appendChild(this.brightnessButton);

            this.colorButton = makeElem(
                'button',
                'color-button device-color-controls-button',
            );
            this.colorButtonSwatch = makeElem('div', 'color-button-swatch');
            this.colorButton.appendChild(this.colorButtonSwatch);
            this.colorButton.addEventListener('click', () => this.editColor());
            this.colorControls.appendChild(this.colorButton);

            this.error = makeElem('label', 'device-error');
            this.error.style.display = 'none';
            this.element.appendChild(this.error);
        }

        updateStatus(status) {
            this.status = status;
            this.element.classList.remove('device-loading');
            this.error.style.display = 'none';
            if (status["is_on"]) {
                this.onOff.classList.add('device-on-off-on');
            } else {
                this.onOff.classList.remove('device-on-off-on');
            }
            this.brightnessButton.textContent = status["brightness"] + "%";
            this.colorButtonSwatch.style.backgroundColor = previewColor(status);
        }

        showError(err) {
            this.error.textContent = err;
            this.error.style.display = 'block';
            this.element.classList.remove('device-loading');
        }

        fetchUpdate() {
            this.doCall(lightAPI.getStatus(this.info.id));
        }

        toggleOnOff() {
            this.doCall(lightAPI.setOnOff(this.info.id, !this.status['is_on']));
        }

        editBrightness() {
            const popup = new window.controlPopups.BrightnessPopup(this.status['brightness']);
            popup.onBrightness = (value) => {
                this.doCall(lightAPI.setBrightness(this.info.id, value));
            };
            popup.open();
        }

        editColor() {
            const popup = new window.controlPopups.ColorPopup(this.status);
            popup.onRGB = (rgb) => {
                // TODO: this.
            };
            popup.onTone = (tone) => {
                // TODO: this.
            };
            popup.open();
        }

        doCall(promise) {
            this.element.classList.add('device-loading');
            promise.then((status) => {
                this.updateStatus(status);
            }).catch((err) => {
                this.showError(err);
            });
        }
    }

    function previewColor(status) {
        if (status['use_rgb']) {
            return rgbToHex(status['rgb']);
        } else {
            const frac = Math.min(100, status['color_tone'] / 100);
            const rgb = TONE_0.map((x0, i) => {
                const x1 = TONE_100[i];
                return Math.round(frac * x1 + (1 - frac) * x0);
            });
            return rgbToHex(rgb);
        }
    }

    window.addEventListener('load', () => {
        window.deviceList = new DeviceList();
        lightAPI.getDevices().then((devs) => {
            window.deviceList.update(devs);
        }).catch((err) => {
            window.deviceList.showError(err);
        })
    });

})();
