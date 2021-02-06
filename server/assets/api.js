(function () {

    class API {
        getDevices() {
            return apiCall('/api/devices?update_status=1');
        }

        async getStatus(deviceID) {
            const encoded = encodeURIComponent(deviceID);
            return (await apiCall('/api/device/status?id=' + encoded))[0];
        }

        async setOnOff(deviceID, on) {
            const encoded = encodeURIComponent(deviceID);
            const onStr = (on ? '1' : '0');
            const url = '/api/device/set_on?id=' + encoded + '&on=' + onStr;
            return (await apiCall(url))[0];
        }

        async setBrightness(deviceID, lum) {
            const encoded = encodeURIComponent(deviceID);
            const url = '/api/device/set_brightness?id=' + encoded + '&brightness=' + lum;
            return (await apiCall(url))[0];
        }

        async setTone(deviceID, tone) {
            const encoded = encodeURIComponent(deviceID);
            const url = '/api/device/set_color_tone?id=' + encoded + '&color_tone=' + tone;
            return (await apiCall(url))[0];
        }

        async setRGB(deviceID, rgb) {
            const [r, g, b] = rgb;
            const encoded = encodeURIComponent(deviceID);
            const url = '/api/device/set_rgb?id=' + encoded + '&r=' + r + '&g=' + g + '&b=' + b;
            return (await apiCall(url))[0];
        }
    }

    async function apiCall(url) {
        return parseRemoteObject(await fetch(url));
    }

    async function parseRemoteObject(obj) {
        const parsed = await obj.json();
        if (parsed.hasOwnProperty('error')) {
            throw new Error(parsed['error']);
        }
        return parsed;
    }

    window.lightAPI = new API();

})();