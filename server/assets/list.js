(function () {

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
            this.brightnessButton = makeElem(
                'button',
                'brightness-button device-color-controls-button',
            );
            this.brightnessButton.addEventListener('click', () => this.editBrightness());
            this.colorControls.appendChild(this.brightnessButton);
            this.element.appendChild(this.colorControls);

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
            // TODO: look at color, brightness, etc.
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

        doCall(promise) {
            this.element.classList.add('device-loading');
            promise.then((status) => {
                this.updateStatus(status);
            }).catch((err) => {
                this.showError(err);
            });
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
