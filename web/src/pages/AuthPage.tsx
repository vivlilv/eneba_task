import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { request } from '../api'
import type { User } from '../api'
import { useApp } from '../appContext'
import { ErrorPanel } from '../components/Feedback'
import { errorMessage } from '../lib/errorMessage'

export function AuthPage() {
  const { onSignedIn } = useApp()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const requestedReturn = searchParams.get('returnTo') || '/'
  const returnTo = requestedReturn.startsWith('/') && !requestedReturn.startsWith('//') ? requestedReturn : '/'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const response = await request<{ user: User }>(`/api/auth/${mode === 'register' ? 'register' : 'login'}`, {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      await onSignedIn(response.user)
      navigate(returnTo, { replace: true })
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="auth-wrap">
      <div className="auth-card">
        <p className="eyebrow">YOUR KEYLANE ACCOUNT</p>
        <h1>{mode === 'login' ? 'Welcome back' : 'Create your account'}</h1>
        <p className="muted">Sign in to continue to checkout and keep your orders in one place.</p>
        {error && <ErrorPanel message={error} />}
        <form onSubmit={submit} className="form-stack">
          <label>Email address<input type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label>
          <label>Password<input type="password" autoComplete={mode === 'register' ? 'new-password' : 'current-password'} minLength={10} required value={password} onChange={(event) => setPassword(event.target.value)} /></label>
          {mode === 'register' && <p className="small-muted">Use at least 10 characters. This demo does not send verification email.</p>}
          <button className="button button-primary button-wide" disabled={busy}>{busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}</button>
        </form>
        <p className="auth-switch">{mode === 'login' ? 'New to Keylane?' : 'Already have an account?'}{' '}
          <button type="button" onClick={() => { setMode(mode === 'login' ? 'register' : 'login'); setError('') }}>{mode === 'login' ? 'Create an account' : 'Sign in'}</button>
        </p>
        <Link className="back-link" to={returnTo === '/checkout' ? '/cart' : '/'}>← Back</Link>
      </div>
    </section>
  )
}
