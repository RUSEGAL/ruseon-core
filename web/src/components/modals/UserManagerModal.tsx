import { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { X, Users, Plus, Trash2, Edit2, Key, Shield, User as UserIcon, ChevronDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface User {
  username: string;
  role: 'admin' | 'operator' | 'viewer' | 'service';
}

interface UserManagerModalProps {
  token: string | null;
  onClose: () => void;
}

interface CustomSelectProps {
  value: string;
  onChange: (val: any) => void;
  options: { label: string, value: string }[];
  style?: React.CSSProperties;
}

function CustomSelect({ value, onChange, options, style }: CustomSelectProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const popupRef = useRef<HTMLDivElement>(null);
  const [rect, setRect] = useState<DOMRect | null>(null);

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (
        ref.current && !ref.current.contains(e.target as Node) &&
        (!popupRef.current || !popupRef.current.contains(e.target as Node))
      ) {
        setOpen(false);
      }
    };
    
    const handleScroll = (e: Event) => {
      if (popupRef.current && popupRef.current.contains(e.target as Node)) {
        return;
      }
      setOpen(false);
    };

    if (open) {
      document.addEventListener('mousedown', handleClick);
      window.addEventListener('scroll', handleScroll, true);
    }
    
    return () => {
      document.removeEventListener('mousedown', handleClick);
      window.removeEventListener('scroll', handleScroll, true);
    };
  }, [open]);

  const handleOpen = () => {
    if (ref.current) {
      setRect(ref.current.getBoundingClientRect());
      setOpen(!open);
    }
  };

  const selected = options.find(o => o.value === value) || options[0];

  return (
    <div ref={ref} style={{ position: 'relative', width: '100%', ...style }}>
      <div 
        className="input-field" 
        style={{ cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}
        onClick={handleOpen}
      >
        <span>{selected?.label}</span>
        <ChevronDown size={16} style={{ color: 'var(--text-muted)' }} />
      </div>
      {open && rect && createPortal(
        <div ref={popupRef} className="glass" style={{
          position: 'fixed', 
          top: rect.bottom + 4, 
          left: rect.left, 
          width: rect.width,
          zIndex: 9999,
          maxHeight: '200px', 
          overflowY: 'auto', 
          padding: '4px', 
          borderRadius: '8px',
          background: 'rgba(26, 31, 51, 0.9)', // ensure opaque
          backdropFilter: 'blur(12px)'
        }}>
          {options.map(opt => (
            <div
              key={opt.value}
              style={{
                padding: '8px 12px', cursor: 'pointer', borderRadius: '6px',
                background: opt.value === value ? 'rgba(255,255,255,0.1)' : 'transparent',
                transition: 'background 0.2s', fontSize: '0.95rem'
              }}
              onMouseEnter={e => e.currentTarget.style.background = 'rgba(255,255,255,0.1)'}
              onMouseLeave={e => e.currentTarget.style.background = opt.value === value ? 'rgba(255,255,255,0.1)' : 'transparent'}
              onClick={(e) => { 
                e.stopPropagation();
                onChange(opt.value); 
                setOpen(false); 
              }}
            >
              {opt.label}
            </div>
          ))}
        </div>,
        document.body
      )}
    </div>
  );
}

export function UserManagerModal({ token, onClose }: UserManagerModalProps) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'admin' | 'operator' | 'viewer' | 'service'>('viewer');
  
  const [editingUsername, setEditingUsername] = useState<string | null>(null);
  const [editPassword, setEditPassword] = useState('');
  const [editRole, setEditRole] = useState<'admin' | 'operator' | 'viewer' | 'service'>('viewer');

  const fetchUsers = async () => {
    try {
      const res = await fetch('/api/users', { headers: { 'Authorization': `Bearer ${token}` } });
      if (!res.ok) throw new Error('Failed to fetch users');
      const data = await res.json();
      setUsers(data || []);
      setError('');
    } catch (err: any) {
      setError(err.message || 'Error fetching users');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUsername.trim() || !newPassword) return;

    try {
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          username: newUsername.trim(),
          password: newPassword,
          role: newRole
        })
      });
      if (res.ok) {
        setNewUsername('');
        setNewPassword('');
        setNewRole('viewer');
        fetchUsers();
      } else {
        const data = await res.json();
        setError(data.error || 'Failed to add user');
      }
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleDelete = async (username: string) => {
    if (!confirm(t('users.deleteConfirm', 'Are you sure you want to delete this user?'))) return;
    try {
      const res = await fetch(`/api/users/${username}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        fetchUsers();
      } else {
        const data = await res.json();
        setError(data.error || 'Failed to delete user');
      }
    } catch (err: any) {
      setError(err.message);
    }
  };

  const startEdit = (u: User) => {
    setEditingUsername(u.username);
    setEditPassword(''); // leave blank if no change
    setEditRole(u.role);
    setError('');
  };

  const cancelEdit = () => {
    setEditingUsername(null);
  };

  const handleSaveEdit = async () => {
    if (!editingUsername) return;
    try {
      const body: any = { role: editRole };
      if (editPassword) body.password = editPassword;

      const res = await fetch(`/api/users/${editingUsername}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(body)
      });
      if (res.ok) {
        setEditingUsername(null);
        fetchUsers();
      } else {
        const data = await res.json();
        setError(data.error || 'Failed to update user');
      }
    } catch (err: any) {
      setError(err.message);
    }
  };

  const roleOptions = [
    { label: t('users.roles.viewer', 'Viewer'), value: 'viewer' },
    { label: t('users.roles.operator', 'Operator'), value: 'operator' },
    { label: t('users.roles.admin', 'Admin'), value: 'admin' },
    { label: t('users.roles.service', 'Service'), value: 'service' }
  ];

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal glass" style={{ maxWidth: '700px', width: '100%', padding: '2rem' }} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '10px' }}>
            <Users size={24} style={{ color: 'var(--primary)' }} />
            {t('users.title', 'User Management')}
          </h2>
          <button className="btn-icon" onClick={onClose}><X size={20} /></button>
        </div>

        {error && (
          <div style={{ padding: '12px', background: 'rgba(255, 68, 68, 0.1)', color: '#ff4444', borderRadius: '8px', marginBottom: '20px', fontSize: '0.9rem' }}>
            {error}
          </div>
        )}

        <div className="glass" style={{ padding: '20px', borderRadius: '12px', marginBottom: '24px' }}>
          <h3 style={{ fontSize: '1rem', marginTop: 0, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
            {t('users.common.add')}
          </h3>
          <form onSubmit={handleAdd} style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '16px', alignItems: 'flex-end' }}>
            <div>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px', display: 'flex', alignItems: 'center', gap: '6px' }}>
                <UserIcon size={14} /> {t('users.username', 'Username')}
              </label>
              <input
                type="text"
                className="input-field"
                value={newUsername}
                onChange={e => setNewUsername(e.target.value)}
                placeholder={t('users.usernamePlaceholder', 'Enter username...')}
                required
              />
            </div>
            <div>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px', display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Key size={14} /> {t('users.password', 'Password')}
              </label>
              <input
                type="password"
                className="input-field"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                placeholder={t('users.passwordPlaceholder', 'Enter password...')}
                required
              />
            </div>
            <div>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px', display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Shield size={14} /> {t('users.role', 'Role')}
              </label>
              <CustomSelect
                value={newRole}
                onChange={val => setNewRole(val as any)}
                options={roleOptions}
              />
            </div>
            <button type="submit" className="btn btn-primary" style={{ height: '42px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px', whiteSpace: 'nowrap' }}>
              <Plus size={18} /> {t('users.common.add')}
            </button>
          </form>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', maxHeight: '400px', overflowY: 'auto', paddingRight: '4px' }}>
          {loading ? (
            <div style={{ textAlign: 'center', padding: '30px', color: 'var(--text-muted)' }}>Loading...</div>
          ) : users.length === 0 ? (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '30px', background: 'rgba(255,255,255,0.02)', borderRadius: '12px' }}>
              {t('users.noUsers', 'No users found.')}
            </div>
          ) : (
            users.map(u => (
              <div key={u.username} style={{ display: 'flex', alignItems: 'center', gap: '16px', padding: '16px', background: 'rgba(255,255,255,0.05)', borderRadius: '12px', transition: 'background 0.2s ease' }} className="hover-highlight">
                <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: '12px' }}>
                  <div style={{ width: '40px', height: '40px', borderRadius: '50%', background: 'rgba(255,255,255,0.1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <UserIcon size={20} style={{ color: 'var(--text-muted)' }} />
                  </div>
                  <div>
                    <div style={{ fontWeight: 600, fontSize: '1.05rem', color: 'var(--text)' }}>{u.username}</div>

                    {editingUsername === u.username ? (
                      <div style={{ display: 'flex', gap: '12px', marginTop: '12px', alignItems: 'center' }}>
                        <input
                          type="password"
                          className="input-field"
                          placeholder={t('users.newPasswordPlaceholder', 'New password (optional)')}
                          value={editPassword}
                          onChange={e => setEditPassword(e.target.value)}
                          style={{ width: '200px' }}
                        />
                        <CustomSelect
                          value={editRole}
                          onChange={val => setEditRole(val as any)}
                          options={roleOptions}
                          style={{ width: '140px' }}
                        />
                        <button className="btn btn-primary" style={{ padding: '6px 12px', fontSize: '0.85rem', whiteSpace: 'nowrap' }} onClick={handleSaveEdit}>{t('users.common.save')}</button>
                        <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: '0.85rem', whiteSpace: 'nowrap' }} onClick={cancelEdit}>{t('users.common.cancel')}</button>
                      </div>
                    ) : (
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginTop: '4px' }}>
                        <Shield size={14} style={{ color: 'var(--primary)' }} />
                        <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                          {t(`users.roles.${u.role}`, u.role)}
                        </span>
                      </div>
                    )}
                  </div>
                </div>

                {editingUsername !== u.username && (
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button
                      className="btn-icon"
                      onClick={() => startEdit(u)}
                      title={t('users.common.edit')}
                    >
                      <Edit2 size={18} />
                    </button>
                    {u.username !== 'admin' && (
                      <button
                        className="btn-icon"
                        style={{ color: '#ff4444' }}
                        onClick={() => handleDelete(u.username)}
                        title={t('users.common.delete')}
                      >
                        <Trash2 size={18} />
                      </button>
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
