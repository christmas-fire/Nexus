let socket = null;
let currentChatID = null;
let currentUserID = null;

function showLoginView() {
    document.getElementById("login-view").classList.remove("d-none");
    document.getElementById("chat-view").classList.add("d-none");
}

function showChatView() {
    document.getElementById("login-view").classList.add("d-none");
    document.getElementById("chat-view").classList.remove("d-none");
}

async function handleLogin(event) {
    event.preventDefault();
    const email = document.getElementById("email").value;
    const password = document.getElementById("password").value;
    try {
        const response = await fetch("/api/v1/login", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ email, password }),
        });
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        localStorage.setItem("authToken", data.access_token);
        connectWebSocket(data.access_token);
    } catch (error) {
        console.error("Login error:", error);
        alert("Login failed.");
    }
}

function connectWebSocket(token) {
    if (!token) return;
    socket = new WebSocket(`ws://${window.location.host}/ws`);
    socket.onopen = () => sendMessageToServer("auth", { token });
    socket.onmessage = handleSocketMessage;
    socket.onerror = (error) => console.error("WebSocket error:", error);
    socket.onclose = () => {
        socket = null;
        showLoginView();
    };
}

function sendMessageToServer(type, payload) {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type, payload }));
    }
}

function handleSocketMessage(event) {
    const msg = JSON.parse(event.data);
    switch (msg.type) {
        case "auth_status":
            if (msg.payload.success) {
                const tokenPayload = JSON.parse(atob(localStorage.getItem("authToken").split('.')[1]));
                currentUserID = parseInt(tokenPayload.sub, 10);
                showChatView();
                sendMessageToServer("get_my_chats", {});
            } else {
                localStorage.removeItem("authToken");
                showLoginView();
            }
            break;
        case "my_chats_list":
            renderChatsList(msg.payload);
            break;
        case "chat_history":
            renderChatHistory(msg.payload);
            break;
        case "new_message":
            if (msg.payload.chat_id === currentChatID) addMessageToChat(msg.payload);
            break;
    }
}

function renderChatsList(payload) {
    const chatsList = document.getElementById("chats-list");
    chatsList.innerHTML = "";
    (payload.chats || []).forEach(chat => {
        const a = document.createElement("a");
        a.href = "#";
        a.className = "list-group-item list-group-item-action";
        a.textContent = chat.name || `Chat ${chat.id.substring(0, 8)}`;
        a.addEventListener("click", (e) => handleChatSelection(e, chat));
        chatsList.appendChild(a);
    });
}

function handleChatSelection(event, chat) {
    event.preventDefault();
    document.querySelectorAll("#chats-list a").forEach(el => el.classList.remove("active"));
    event.currentTarget.classList.add("active");
    currentChatID = chat.id;
    document.getElementById("chat-title").textContent = chat.name || `Chat ${chat.id.substring(0, 8)}`;
    document.getElementById("messages-container").innerHTML = "";
    document.getElementById("message-input").disabled = false;
    document.querySelector("#message-form button").disabled = false;
    document.getElementById("message-input").focus();

    sendMessageToServer("get_chat_history", { chat_id: currentChatID });
}

function renderChatHistory(payload) {
    document.getElementById("messages-container").innerHTML = "";
    (payload.messages || []).forEach(msg => {
        addMessageToChat(msg, false);
    });
}

function addMessageToChat(msg) {
    const messagesContainer = document.getElementById("messages-container");
    const messageElement = document.createElement("div");
    const isMyMessage = msg.sender_id === currentUserID;
    const alignClass = isMyMessage ? 'ms-auto' : 'me-auto';
    const bgClass = isMyMessage ? 'bg-primary text-white' : 'bg-light text-dark border';
    messageElement.innerHTML = `
        <div class="card w-75 mb-2 ${alignClass}" style="max-width: 75%;">
            <div class="card-body p-2">
                <strong class="card-title">User ${msg.sender_id}</strong>
                <p class="mb-0">${msg.text}</p>
                <small class="text-white-50 text-end d-block">${new Date(msg.sent_at).toLocaleTimeString()}</small>
            </div>
        </div>`;
    messagesContainer.prepend(messageElement);
}

document.getElementById("login-form").addEventListener("submit", handleLogin);
document.getElementById("register-btn").addEventListener("click", () => alert("Register not implemented yet."));

document.getElementById("message-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const text = document.getElementById("message-input").value;
    if (text && socket && currentChatID) {
        sendMessageToServer("send_message", { chat_id: currentChatID, text: text });
        document.getElementById("message-input").value = "";
    }
});

const savedToken = localStorage.getItem("authToken");
if (savedToken) {
    connectWebSocket(savedToken);
}