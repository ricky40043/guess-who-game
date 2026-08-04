const SESSION_KEY = 'guess_who_session'
const defaultSettings = { questionCount: 5, answerSeconds: 60, guessSeconds: 120, questionMode: 'random', questionIds: [], customTexts: [] }

const state = {
  connected: false, roomId: '', playerId: '', playerName: '', hostToken: '', isHost: false,
  status: 'home', settings: { ...defaultSettings }, players: [], bank: [], question: null,
  questionIndex: 0, questionNumber: 0, totalQuestions: 0, timeLeft: 0, submittedCount: 0,
  hasSubmitted: false, myAnswer: '', reveal: null, revealComplete: false, aliases: [],
  guessPlayers: [], ownAlias: '', ownPlayerId: '', guesses: {}, hasGuessed: false,
  results: [], identities: [],
}

let socket
let reconnectTimer
let toastTimer

function wsURL() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${location.host}/ws`
}

function connect() {
  clearTimeout(reconnectTimer)
  socket = new WebSocket(wsURL())
  socket.onopen = () => {
    state.connected = true
    updateConnectionStatus()
    const session = loadSession()
    if (session?.roomId) send('REJOIN_ROOM', session)
  }
  socket.onclose = () => {
    state.connected = false
    updateConnectionStatus()
    reconnectTimer = setTimeout(connect, 1800)
  }
  socket.onmessage = event => {
    try { handleMessage(JSON.parse(event.data)) } catch (error) { console.error(error) }
  }
}

function send(type, data = {}) {
  if (socket?.readyState !== WebSocket.OPEN) return toast('連線尚未建立')
  socket.send(JSON.stringify({ type, data }))
}

function loadSession() {
  try { return JSON.parse(localStorage.getItem(SESSION_KEY) || 'null') } catch { return null }
}
function saveSession() {
  localStorage.setItem(SESSION_KEY, JSON.stringify({ roomId: state.roomId, playerId: state.playerId, hostToken: state.hostToken }))
}
function clearSession() { localStorage.removeItem(SESSION_KEY) }

function toast(message) {
  const el = document.querySelector('#toast')
  if (!el) return
  el.textContent = message
  el.classList.add('show')
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => el.classList.remove('show'), 2400)
}

function mergeRoom(data) {
  state.roomId = data.roomId || state.roomId
  state.status = data.status || state.status
  state.settings = data.settings || state.settings
  state.players = data.players || state.players
  state.isHost = Boolean(data.isHost ?? state.isHost)
  state.playerId = data.playerId || state.playerId
  state.playerName = data.playerName || state.playerName
  state.hostToken = data.hostToken || state.hostToken
}

function applyQuestion(data) {
  state.status = 'answering'
  state.question = data.question
  state.questionIndex = data.questionIndex
  state.questionNumber = data.questionNumber
  state.totalQuestions = data.totalQuestions
  state.timeLeft = data.answerSeconds ?? Math.max(0, Math.ceil((data.deadlineAt - Date.now()) / 1000))
  state.submittedCount = data.submittedCount || 0
  state.hasSubmitted = Boolean(data.hasSubmitted)
  state.myAnswer = data.myAnswer || ''
}

function applyGuessing(data) {
  state.status = 'guessing'
  state.aliases = data.aliases || []
  state.guessPlayers = data.players || []
  state.ownAlias = data.ownAlias || ''
  state.ownPlayerId = data.ownPlayerId || ''
  state.timeLeft = data.guessSeconds ?? Math.max(0, Math.ceil((data.deadlineAt - Date.now()) / 1000))
  state.submittedCount = data.submittedCount || 0
  state.hasGuessed = Boolean(data.hasGuessed)
  state.guesses = {}
}

function applySnapshot(data) {
  mergeRoom(data)
  switch (data.status) {
    case 'waiting': break
    case 'answering': applyQuestion(data); break
    case 'revealing':
      state.status = 'revealing'; state.reveal = data.reveal || null; state.revealComplete = Boolean(data.revealComplete); break
    case 'guessing': applyGuessing(data); break
    case 'finished':
      state.status = 'finished'; state.results = data.results || []; state.identities = data.identities || []; break
  }
}

function updateConnectionStatus() {
  const el = document.querySelector('.connection')
  if (!el) return
  el.textContent = state.connected ? '● 已連線' : '● 重新連線中'
  el.classList.toggle('online', state.connected)
}

function updateLiveStatus() {
  const timer = document.querySelector('#countdown')
  if (timer) timer.textContent = state.timeLeft
  const total = state.players.length
  const ratio = total ? Math.round(state.submittedCount / total * 100) : 0
  const progress = document.querySelector('#progress-bar')
  if (progress) progress.style.width = `${ratio}%`
  const text = document.querySelector('#progress-text')
  if (text) text.textContent = `${state.submittedCount} / ${total} 人已提交`
}

function updateAnswerAccepted(answer) {
  state.hasSubmitted = true
  state.myAnswer = answer
  const button = document.querySelector('#submit-answer')
  if (button) button.textContent = '修改答案'
  toast('答案已送出，可在時間內修改')
}

function updateWaitingPlayers() {
  if (state.status !== 'waiting') return
  const players = document.querySelector('#waiting-players')
  const count = document.querySelector('#waiting-player-count')
  if (count) count.textContent = `玩家 ${state.players.length} 人`
  if (players) players.innerHTML = state.players.map(p => `<span class="player-pill ${p.connected ? '' : 'offline'}">${p.connected ? '🟢' : '⚪'} ${esc(p.name)}</span>`).join('') || '<span class="muted">等待玩家加入…</span>'
}

function handleMessage(message) {
  const data = message.data || {}
  switch (message.type) {
    case 'ROOM_CREATED':
      mergeRoom({ ...data, isHost: true }); state.playerId = ''; saveSession(); toast('房間建立完成'); render(); return
    case 'JOIN_SUCCESS':
      applySnapshot({ ...data, isHost: false }); saveSession(); toast(`歡迎，${state.playerName}`); render(); return
    case 'REJOIN_SUCCESS':
      applySnapshot(data); saveSession(); toast('已恢復連線'); render(); return
    case 'PLAYER_JOINED':
    case 'PLAYER_REJOINED':
    case 'PLAYER_DISCONNECTED':
      state.players = data.players || []
      updateWaitingPlayers()
      updateLiveStatus()
      return
    case 'SETTINGS_UPDATED':
      state.settings = data.settings
      toast('設定已更新')
      if (state.status === 'waiting') render()
      return
    case 'GAME_STARTED': return
    case 'QUESTION_STARTED': applyQuestion(data); render(); return
    case 'ANSWER_ACCEPTED': updateAnswerAccepted(data.answer); return
    case 'ANSWER_PROGRESS': state.submittedCount = data.submittedCount || 0; updateLiveStatus(); return
    case 'REVEAL_STARTED':
    case 'PROFILE_REVEALED':
      state.status = 'revealing'; state.reveal = data; state.revealComplete = false; render(); return
    case 'REVEAL_COMPLETE': state.revealComplete = true; render(); return
    case 'GUESSING_STARTED': applyGuessing(data); render(); return
    case 'GUESS_ACCEPTED':
      state.hasGuessed = true
      document.querySelector('#submit-guesses')?.setAttribute('disabled', '')
      toast('配對已送出')
      return
    case 'GUESS_PROGRESS': state.submittedCount = data.submittedCount || 0; updateLiveStatus(); return
    case 'TIMER_UPDATE': state.timeLeft = data.timeLeft; updateLiveStatus(); return
    case 'GAME_FINISHED':
      state.status = 'finished'; state.results = data.results || []; state.identities = data.identities || []; render(); return
    case 'HOST_DISCONNECTED': toast('房主畫面斷線，等待重新連線'); return
    case 'HOST_RECONNECTED': toast('房主已恢復連線'); return
    case 'ROOM_CLOSED':
      clearSession(); resetState(); toast(data.message || '房間已關閉'); render(); return
    case 'ERROR':
      if (data.code === 'REJOIN_FAILED') { clearSession(); resetState(); render() }
      toast(data.message || '操作失敗')
      return
  }
}

function resetState() {
  Object.assign(state, {
    roomId: '', playerId: '', playerName: '', hostToken: '', isHost: false, status: 'home',
    settings: { ...defaultSettings }, players: [], question: null, results: [], identities: [],
  })
}

const esc = (value = '') => String(value).replace(/[&<>'"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[c]))
const app = document.querySelector('#app')
const connection = () => `<span class="connection ${state.connected ? 'online' : ''}">${state.connected ? '● 已連線' : '● 重新連線中'}</span>`
const header = () => `<div class="status-bar"><strong>猜人</strong>${connection()}</div>`

function homeView() {
  const roomParam = new URLSearchParams(location.search).get('room') || ''
  return `<div class="shell">
    <section class="hero"><div class="logo"><span class="logo-badge">🕵️</span>猜人</div><p class="subtitle">回答生活題，匿名公開，再猜出每一個人。</p></section>
    <div class="grid two">
      <section class="card"><h2>建立房間</h2><p class="muted">主畫面建議開在電視或筆電。房主不算玩家，請另外用手機加入。</p><button class="btn large" id="create-room">建立公開遊戲房間</button></section>
      <section class="card"><h2>加入遊戲</h2>
        <div class="field"><label>房間代碼</label><input id="join-room-id" maxlength="6" value="${esc(roomParam.toUpperCase())}" placeholder="例如 ABC234" /></div>
        <div class="field"><label>暱稱（必填，不可重複）</label><input id="join-name" maxlength="12" placeholder="輸入 1～12 字暱稱" /></div>
        <button class="btn large" id="join-room">加入房間</button>
      </section>
    </div>
  </div>`
}

function lobbyView() {
  const joinURL = `${location.origin}/?room=${state.roomId}`
  const qrURL = `https://api.qrserver.com/v1/create-qr-code/?size=240x240&margin=8&data=${encodeURIComponent(joinURL)}`
  const playerPills = state.players.map(p => `<span class="player-pill ${p.connected ? '' : 'offline'}">${p.connected ? '🟢' : '⚪'} ${esc(p.name)}</span>`).join('') || '<span class="muted">等待玩家加入…</span>'
  const settings = state.settings
  const options = state.bank.map(q => `<label class="question-option"><input type="checkbox" class="question-check" value="${q.id}" ${settings.questionIds?.includes(q.id) ? 'checked' : ''}/><span><strong>${esc(q.text)}</strong><br><small class="muted">${esc(q.category)}</small></span></label>`).join('')
  return `<div class="shell">${header()}<div class="grid two">
    <section class="card center"><h2>掃描 QR Code 加入</h2><img class="qr-code" src="${qrURL}" alt="掃描加入房間 ${esc(state.roomId)}" referrerpolicy="no-referrer"><div class="room-code">${esc(state.roomId)}</div><p class="muted">掃描後輸入暱稱即可加入，也可以手動輸入房號。</p><div class="actions" style="justify-content:center"><button class="btn secondary" id="copy-link">複製加入連結</button></div><hr style="border-color:#26364d;margin:24px 0"><h3 id="waiting-player-count">玩家 ${state.players.length} 人</h3><div class="players" id="waiting-players">${playerPills}</div></section>
    <section class="card">
      ${state.isHost ? `<h2>遊戲設定</h2>
        <div class="grid two"><div class="field"><label>每題作答秒數</label><input id="answer-seconds" type="number" min="15" max="300" value="${settings.answerSeconds}" /></div><div class="field"><label>猜人秒數</label><input id="guess-seconds" type="number" min="30" max="600" value="${settings.guessSeconds}" /></div></div>
        <div class="field"><label>出題方式</label><select id="question-mode"><option value="random" ${settings.questionMode === 'random' ? 'selected' : ''}>隨機抽題</option><option value="custom" ${settings.questionMode === 'custom' ? 'selected' : ''}>自選題目 / 自訂題目</option></select></div>
        <div id="random-settings" class="field ${settings.questionMode === 'custom' ? 'hidden' : ''}"><label>題目數量</label><input id="question-count" type="number" min="1" max="20" value="${settings.questionCount}" /></div>
        <div id="custom-settings" class="${settings.questionMode === 'custom' ? '' : 'hidden'}"><label>勾選題庫</label><div class="question-list">${options}</div><div class="field"><label>自訂題目（每行一題）</label><textarea id="custom-texts" placeholder="例如：最想刪掉哪一張舊照片？">${esc((settings.customTexts || []).join('\n'))}</textarea></div></div>
        <div class="actions"><button class="btn secondary" id="save-settings">儲存設定</button><button class="btn large" id="start-game">開始遊戲</button></div>` : `<h2>等待房主開始</h2><p class="muted">遊戲開始後不能再加入。請保持此頁開啟。</p>`}
    </section>
  </div></div>`
}

function answeringView() {
  const ratio = state.players.length ? Math.round(state.submittedCount / state.players.length * 100) : 0
  return `<div class="shell">${header()}<section class="card center">
    <div class="timer" id="countdown">${state.timeLeft}</div><div class="muted">第 ${state.questionNumber} / ${state.totalQuestions} 題</div>
    <div class="question">${esc(state.question?.text)}</div>
    <div class="progress"><span id="progress-bar" style="width:${ratio}%"></span></div><p class="muted" id="progress-text">${state.submittedCount} / ${state.players.length} 人已提交</p>
    ${state.isHost ? `<p>所有人提交後會自動進入下一題。</p>` : `<div class="field" style="max-width:700px;margin:20px auto"><textarea id="answer-input" maxlength="200" enterkeyhint="done" placeholder="輸入你的答案…">${esc(state.myAnswer)}</textarea></div><button class="btn large" id="submit-answer">${state.hasSubmitted ? '修改答案' : '送出答案'}</button>`}
  </section></div>`
}

function revealingView() {
  const detail = state.reveal?.profile
  const answers = detail?.answers?.map(item => `<div class="answer-row"><div class="q">${esc(item.question)}</div><div class="a">${esc(item.answer)}</div></div>`).join('') || ''
  return `<div class="shell">${header()}<section class="card">
    ${state.revealComplete ? `<div class="center"><h2>所有匿名答案已公布</h2><p class="muted">下一步開始猜人配對。</p>${state.isHost ? '<button class="btn large" id="start-guessing">開始猜人</button>' : '<p>等待房主開始猜人階段…</p>'}</div>` : `<div class="muted center">第 ${state.reveal?.revealNumber || 1} / ${state.reveal?.totalProfiles || state.players.length} 位</div><h1 class="profile-title">${esc(detail?.alias)}</h1><div class="answer-stack">${answers}</div>${state.isHost ? `<div class="actions" style="justify-content:center"><button class="btn large" id="next-reveal">${state.reveal?.isLast ? '公布完成' : '下一位'}</button></div>` : '<p class="muted center">等待房主切換下一位…</p>'}`}
  </section></div>`
}

function guessingView() {
  const activeAliases = state.aliases.filter(alias => alias !== state.ownAlias)
  const availablePlayers = state.guessPlayers.filter(player => player.id !== state.ownPlayerId)
  const rows = activeAliases.map(alias => {
    const options = ['<option value="">選擇玩家</option>', ...availablePlayers.map(player => `<option value="${player.id}" ${state.guesses[alias] === player.id ? 'selected' : ''}>${esc(player.name)}</option>`)].join('')
    return `<div class="match-row"><div class="match-alias">${esc(alias)}</div><select class="guess-select" data-alias="${esc(alias)}" ${state.hasGuessed ? 'disabled' : ''}>${options}</select></div>`
  }).join('')
  const ratio = state.players.length ? Math.round(state.submittedCount / state.players.length * 100) : 0
  return `<div class="shell">${header()}<section class="card"><div class="timer" id="countdown">${state.timeLeft}</div><h2 class="center">把匿名答案配對到正確玩家</h2><p class="muted center">自己的「${esc(state.ownAlias || '匿名代號')}」與自己的名字已排除。</p><div class="progress"><span id="progress-bar" style="width:${ratio}%"></span></div><p class="muted center" id="progress-text">${state.submittedCount} / ${state.players.length} 人已提交</p>
    ${state.isHost ? '<p class="center">玩家正在手機上完成配對。</p>' : `<div class="match-list">${rows}</div><div class="actions" style="justify-content:center"><button class="btn large" id="submit-guesses" ${state.hasGuessed ? 'disabled' : ''}>${state.hasGuessed ? '已提交，等待其他人' : '提交全部配對'}</button></div>`}
  </section></div>`
}

function finishedView() {
  const ranks = state.results.map(r => `<div class="rank-row"><div class="rank">#${r.rank}</div><div><strong>${esc(r.playerName)}</strong><div class="muted">猜中 ${r.correct} / ${r.possible}${r.perfectBonus ? '，全對獎勵 +2' : ''}</div></div><strong>${r.score} 分</strong></div>`).join('')
  const identities = state.identities.map(i => `<div class="identity"><strong>${esc(i.alias)}</strong><br>${esc(i.playerName)}</div>`).join('')
  return `<div class="shell">${header()}<div class="grid two"><section class="card"><h2>🏆 最終排名</h2><div class="rank-list">${ranks}</div></section><section class="card"><h2>正確答案</h2><div class="identities">${identities}</div><div class="actions"><button class="btn secondary" id="back-home">回首頁</button></div></section></div></div>`
}

function render() {
  if (!state.roomId || state.status === 'home') app.innerHTML = homeView()
  else if (state.status === 'waiting') app.innerHTML = lobbyView()
  else if (state.status === 'answering') app.innerHTML = answeringView()
  else if (state.status === 'revealing') app.innerHTML = revealingView()
  else if (state.status === 'guessing') app.innerHTML = guessingView()
  else if (state.status === 'finished') app.innerHTML = finishedView()
  bindEvents()
}

function bindEvents() {
  document.querySelector('#create-room')?.addEventListener('click', () => send('CREATE_ROOM', { settings: defaultSettings }))
  document.querySelector('#join-room')?.addEventListener('click', () => {
    const roomId = document.querySelector('#join-room-id').value.trim().toUpperCase()
    const name = document.querySelector('#join-name').value.trim()
    if (!roomId || !name) return toast('請輸入房間代碼與暱稱')
    send('JOIN_ROOM', { roomId, name })
  })
  document.querySelector('#copy-link')?.addEventListener('click', async () => {
    await navigator.clipboard.writeText(`${location.origin}/?room=${state.roomId}`)
    toast('加入連結已複製')
  })
  document.querySelector('#question-mode')?.addEventListener('change', event => {
    document.querySelector('#random-settings').classList.toggle('hidden', event.target.value === 'custom')
    document.querySelector('#custom-settings').classList.toggle('hidden', event.target.value !== 'custom')
  })
  document.querySelector('#save-settings')?.addEventListener('click', saveSettings)
  document.querySelector('#start-game')?.addEventListener('click', () => send('START_GAME'))
  document.querySelector('#answer-input')?.addEventListener('input', event => { state.myAnswer = event.target.value })
  document.querySelector('#submit-answer')?.addEventListener('click', () => {
    const answer = document.querySelector('#answer-input').value.trim()
    if (!answer) return toast('答案不能空白')
    send('SUBMIT_ANSWER', { questionIndex: state.questionIndex, answer })
  })
  document.querySelector('#next-reveal')?.addEventListener('click', () => send('NEXT_REVEAL'))
  document.querySelector('#start-guessing')?.addEventListener('click', () => send('START_GUESSING'))
  document.querySelectorAll('.guess-select').forEach(select => select.addEventListener('change', event => {
    const alias = event.target.dataset.alias
    const playerId = event.target.value
    if (playerId) {
      for (const [otherAlias, target] of Object.entries(state.guesses)) if (otherAlias !== alias && target === playerId) state.guesses[otherAlias] = ''
    }
    state.guesses[alias] = playerId
    render()
  }))
  document.querySelector('#submit-guesses')?.addEventListener('click', () => {
    const required = state.aliases.filter(alias => alias !== state.ownAlias)
    if (required.some(alias => !state.guesses[alias])) return toast('請完成所有配對')
    if (new Set(required.map(alias => state.guesses[alias])).size !== required.length) return toast('每個名字只能使用一次')
    send('SUBMIT_GUESSES', { guesses: state.guesses })
  })
  document.querySelector('#back-home')?.addEventListener('click', () => { clearSession(); resetState(); history.replaceState({}, '', '/'); render() })
}

function saveSettings() {
  const mode = document.querySelector('#question-mode').value
  const questionIds = [...document.querySelectorAll('.question-check:checked')].map(input => Number(input.value))
  const customTexts = (document.querySelector('#custom-texts')?.value || '').split('\n').map(v => v.trim()).filter(Boolean)
  send('UPDATE_SETTINGS', {
    questionCount: Number(document.querySelector('#question-count')?.value || 5),
    answerSeconds: Number(document.querySelector('#answer-seconds').value),
    guessSeconds: Number(document.querySelector('#guess-seconds').value),
    questionMode: mode, questionIds, customTexts,
  })
}

fetch('/api/questions').then(r => r.json()).then(data => {
  state.bank = data.questions || []
  if (state.status === 'home' || state.status === 'waiting') render()
}).catch(() => {})
render()
connect()
