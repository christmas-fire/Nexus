import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';

export default function LoginPage() {
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');

    const navigate = useNavigate();

    const handleLogin = async (event) => {
        event.preventDefault();
        setError('');
        setSuccess('');

        try {
            const response = await axios.post('/api/v1/login', {
                email: email,
                password: password,
            });

            localStorage.setItem('authToken', response.data.access_token);
            navigate('/');

        } catch (err) {
            console.error('Login failed:', err);
            setError('Login failed. Please check your credentials.');
        }
    };
    
    const handleRegister = async () => {
        setError('');
        setSuccess('');

        try {
            await axios.post('/api/v1/register', {
                email: email,
                username: email,
                password: password,
            });

            setSuccess('Registration successful! Please log in with your new account.');
            setEmail('');
            setPassword('');

        } catch (err) {
            console.error('Registration failed:', err);
            setError(err.response?.data?.message || 'Registration failed.');
        }
    };

    return (
        <div className="row justify-content-center">
            <div className="col-md-6 col-lg-4" style={{ marginTop: '15vh' }}>
                <div className="card">
                    <div className="card-body">
                        <h3 className="card-title text-center mb-4">Nexus Messenger</h3>
                        <form onSubmit={handleLogin}>
                            <div className="mb-3">
                                <label htmlFor="email" className="form-label">Email</label>
                                <input 
                                    type="email" 
                                    className="form-control" 
                                    id="email"
                                    value={email}
                                    onChange={(e) => setEmail(e.target.value)}
                                    required 
                                />
                            </div>
                            <div className="mb-3">
                                <label htmlFor="password" className="form-label">Password</label>
                                <input 
                                    type="password" 
                                    className="form-control" 
                                    id="password" 
                                    value={password}
                                    onChange={(e) => setPassword(e.target.value)}
                                    required 
                                />
                            </div>

                            {success && <div className="alert alert-success">{success}</div>}
                            {error && <div className="alert alert-danger">{error}</div>}
                            
                            <div className="d-grid gap-2">
                                <button type="submit" className="btn btn-primary">Login</button>
                                <button type="button" className="btn btn-secondary" onClick={handleRegister}>
                                    Register
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        </div>
    );
}
