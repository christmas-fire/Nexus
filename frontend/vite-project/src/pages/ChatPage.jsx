import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { wsService } from '../services/websocket';

export default function ChatPage() {
    const [chats, setChats] = useState([]);
    const [messages, setMessages] = useState([]);
    const [currentChat, setCurrentChat] = useState(null);
    const [newMessage, setNewMessage] = useState('');
    const [currentUserID, setCurrentUserID] = useState(null);
    const navigate = useNavigate();
    const messagesContainerRef = useRef(null);

    useEffect(() => {
        const token = localStorage.getItem('authToken');

        const tokenPayload = JSON.parse(atob(token.split('.')[1]));
        setCurrentUserID(parseInt(tokenPayload.sub, 10));
        wsService.connect(token);

        const handleAuthStatus = (payload) => {
            if (payload.success) wsService.sendMessage('get_my_chats', {});
            else { localStorage.removeItem('authToken'); navigate('/login'); }
        };
        const handleChatsList = (payload) => setChats(payload.chats || []);
        const handleChatHistory = (payload) => setMessages(payload.messages || []);

        wsService.on('auth_status', handleAuthStatus);
        wsService.on('my_chats_list', handleChatsList);
        wsService.on('chat_history', handleChatHistory);

        return () => {
            wsService.off('auth_status', handleAuthStatus);
            wsService.off('my_chats_list', handleChatsList);
            wsService.off('chat_history', handleChatHistory);
        };
    }, [navigate]);

    useEffect(() => {
        if (!currentChat) return;

        const handleNewMessage = (payload) => {
            if (payload.chat_id === currentChat.id) {
                setMessages(prev => [payload, ...prev]);
            }
        };

        wsService.on('new_message', handleNewMessage);

        return () => {
            wsService.off('new_message', handleNewMessage);
        };
    }, [currentChat]);

    const handleSelectChat = (chat) => {
        setCurrentChat(chat);
        setMessages([]);
        wsService.sendMessage('get_chat_history', { chat_id: chat.id });
    };

    const handleSendMessage = (e) => {
        e.preventDefault();
        if (newMessage.trim() && currentChat) {
            wsService.sendMessage('send_message', {
                chat_id: currentChat.id,
                text: newMessage,
            });
            setNewMessage('');
        }
    };

    return (
        <div className="row vh-100 py-3">
            <div className="col-4 h-100 d-flex flex-column">
                <div className="card flex-grow-1">
                    <div className="card-header"><h5>My Chats</h5></div>
                    <ul className="list-group list-group-flush overflow-auto">
                        {chats.map(chat => (
                            <a href="#" key={chat.id} onClick={() => handleSelectChat(chat)}
                               className={`list-group-item list-group-item-action ${currentChat?.id === chat.id ? 'active' : ''}`}>
                                {chat.name || `Chat ${chat.id.substring(0, 8)}`}
                            </a>
                        ))}
                    </ul>
                </div>
            </div>
            <div className="col-8 h-100">
                <div className="card h-100 d-flex flex-column">
                    <div className="card-header"><h5>{currentChat ? (currentChat.name || `Chat ${currentChat.id.substring(0, 8)}`) : 'Select a chat'}</h5></div>
                    <div 
                        ref={messagesContainerRef}
                        className="card-body flex-grow-1" 
                        style={{ overflowY: 'auto' }}
                    >
                        <div>
                            {messages.map(msg => {
                                const isMyMessage = msg.sender_id === currentUserID;
                                const alignClass = isMyMessage ? 'ms-auto' : 'me-auto';
                                const bgClass = isMyMessage ? 'bg-primary text-white' : 'bg-light text-dark border';
                                return (
                                    <div key={msg.id} className={`w-75 mb-2 ${alignClass}`} style={{ maxWidth: '75%' }}>
                                        <div className={`card ${bgClass}`}>
                                            <div className="card-body p-2">
                                                <strong className="card-title">User {msg.sender_id}</strong>
                                                <p className="mb-0">{msg.text}</p>
                                                <small className="d-block text-end" style={{ opacity: 0.7 }}>
                                                    {new Date(msg.sent_at).toLocaleTimeString()}
                                                </small>
                                            </div>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    </div>
                    <div className="card-footer">
                        <form onSubmit={handleSendMessage} className="d-flex">
                            <input type="text" className="form-control" value={newMessage} onChange={(e) => setNewMessage(e.target.value)}
                                   placeholder="Type a message..." disabled={!currentChat} />
                            <button type="submit" className="btn btn-primary ms-2" disabled={!currentChat}>Send</button>
                        </form>
                    </div>
                </div>
            </div>
        </div>
    );
}
