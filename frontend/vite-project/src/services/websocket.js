class WebSocketService {
    constructor() {
        this.socket = null;
        this.listeners = new Map();
    }

    connect(token) {
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            console.log("WebSocket is already connected.");
            return;
        }

        const url = `ws://${window.location.host}/ws`;
        console.log("Connecting to WebSocket at:", url);
        this.socket = new WebSocket(url);

        this.socket.onopen = () => {
            console.log("WebSocket connection established.");
            this.sendMessage("auth", { token });
        };

        this.socket.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            if (this.listeners.has(msg.type)) {
                this.listeners.get(msg.type).forEach(callback => callback(msg.payload));
            }
        };

        this.socket.onerror = (error) => console.error("WebSocket error:", error);
        this.socket.onclose = () => console.log("WebSocket connection closed.");
    }

    on(eventType, callback) {
        if (!this.listeners.has(eventType)) {
            this.listeners.set(eventType, []);
        }
        this.listeners.get(eventType).push(callback);
    }

    sendMessage(type, payload) {
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(JSON.stringify({ type, payload }));
        }
    }

    off(eventType, callback) {
        if (this.listeners.has(eventType)) {
            const updatedListeners = this.listeners.get(eventType)
                .filter(cb => cb !== callback);
            this.listeners.set(eventType, updatedListeners);
        }
    }
}

export const wsService = new WebSocketService();
