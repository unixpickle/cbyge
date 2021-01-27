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

            const errorElem = document.createElement('div');
            errorElem.className = 'devices-error';
            errorElem.textContent = err;
            this.element.appendChild(errorElem);
        }
    }

    class Device {
        constructor(info) {
            this.info = info;
            this.status = null;

            this.element = document.createElement('div');
            this.element.className = 'device';

            this.name = document.createElement('label');
            this.name.className = 'device-name';
            this.name.textContent = info.name;
            this.element.appendChild(this.name);

            this.onOff = document.createElement('div');
            this.onOff.className = 'device-on-off';
            this.element.appendChild(this.onOff);
            this.onOff.addEventListener('click', () => this.toggleOnOff());

            this.colorControls = document.createElement('div');
            this.colorControls.className = 'device-color-controls';
            this.brightnessButton = document.createElement('button');
            this.brightnessButton.className = 'brightness-button device-color-controls-button';
            this.colorControls.appendChild(this.brightnessButton);
            this.element.appendChild(this.colorControls);

            this.error = document.createElement('label');
            this.error.className = 'device-error';
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
