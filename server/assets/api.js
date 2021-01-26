(function () {

    class API {
        getDevices() {
            return apiCall('/api/devices');
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