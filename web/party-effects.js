(() => {
  const originalHandleMessage = handleMessage
  const originalRender = render
  let confettiRunning = false
  let lastFinishedSignature = ''
  let previousQuestionKey = ''

  handleMessage = function (message) {
    originalHandleMessage(message)
    const type = message?.type

    if (type === 'TIMER_UPDATE') updateUrgency()
    if (type === 'QUESTION_STARTED') animateQuestionChange()
    if (type === 'REVEAL_STARTED' || type === 'PROFILE_REVEALED') animateReveal()
    if (type === 'GAME_FINISHED') queueMicrotask(showResultsCelebration)
  }

  render = function () {
    originalRender()
    enhancePartyUI()
  }

  function enhancePartyUI() {
    document.body.classList.toggle('party-finished', state.status === 'finished')
    updateUrgency()

    if (state.status === 'answering') {
      const key = `${state.questionIndex}:${state.question?.id || state.question?.text || ''}`
      if (key !== previousQuestionKey) {
        previousQuestionKey = key
        animateQuestionChange()
      }
    }

    if (state.status === 'revealing') animateReveal()
    if (state.status === 'finished') showResultsCelebration()
  }

  function updateUrgency() {
    const timer = document.querySelector('#countdown')
    if (!timer) return
    const urgent = Number(state.timeLeft) <= 10 && Number(state.timeLeft) > 0
    const critical = Number(state.timeLeft) <= 3 && Number(state.timeLeft) > 0
    timer.classList.toggle('timer-urgent', urgent)
    timer.classList.toggle('timer-critical', critical)
    document.body.classList.toggle('countdown-urgent', urgent)
  }

  function animateQuestionChange() {
    const card = document.querySelector('.answer-stage-card, .card.center')
    const question = document.querySelector('.question')
    if (!card || !question) return
    card.classList.remove('question-flash')
    question.classList.remove('question-enter')
    void card.offsetWidth
    card.classList.add('question-flash')
    question.classList.add('question-enter')
    window.setTimeout(() => card.classList.remove('question-flash'), 850)
  }

  function animateReveal() {
    const card = document.querySelector('.reveal-stage-card, .profile-title')?.closest('.card')
    if (!card || card.dataset.partyReveal === String(state.reveal?.revealNumber || '')) return
    card.dataset.partyReveal = String(state.reveal?.revealNumber || '')
    card.classList.remove('reveal-spotlight')
    void card.offsetWidth
    card.classList.add('reveal-spotlight')
  }

  function seconds(ms) {
    const value = Number(ms || 0) / 1000
    return `${value.toFixed(1)} 秒`
  }

  function showResultsCelebration() {
    if (!Array.isArray(state.results) || !state.results.length) return
    const signature = state.results.map(result => `${result.playerId}:${result.rank}:${result.score}:${result.durationMs}`).join('|')
    decorateResults()
    if (signature === lastFinishedSignature) return
    lastFinishedSignature = signature

    const winner = state.results[0]
    const isWinnerPhone = Boolean(state.playerId && winner?.playerId === state.playerId)
    launchConfetti(isWinnerPhone ? 5200 : 2800, isWinnerPhone ? 150 : 85)
    if (isWinnerPhone) showWinnerToast(winner)
  }

  function decorateResults() {
    const winner = state.results[0]
    const shell = document.querySelector('.shell')
    if (!shell || !winner) return

    let banner = document.querySelector('#champion-banner')
    if (!banner) {
      banner = document.createElement('section')
      banner.id = 'champion-banner'
      banner.className = 'champion-banner'
      const firstCard = shell.querySelector('.grid')
      shell.insertBefore(banner, firstCard || shell.firstChild)
    }
    banner.innerHTML = `
      <div class="champion-crown">👑</div>
      <div class="champion-copy">
        <div class="champion-label">本局第一名</div>
        <div class="champion-name">${esc(winner.playerName)}</div>
        <div class="champion-meta">${winner.score} 分・猜中 ${winner.correct}/${winner.possible}・${seconds(winner.durationMs)}</div>
      </div>
    `

    const rows = document.querySelectorAll('.rank-row')
    rows.forEach((row, index) => {
      const result = state.results[index]
      if (!result) return
      row.classList.toggle('rank-winner', index === 0)
      row.style.setProperty('--rank-delay', `${index * 90}ms`)
      let timing = row.querySelector('.rank-timing')
      if (!timing) {
        timing = document.createElement('div')
        timing.className = 'rank-timing'
        row.querySelector('div:nth-child(2)')?.appendChild(timing)
      }
      timing.textContent = `提交時間 ${seconds(result.durationMs)}`
    })

    const rankList = document.querySelector('.rank-list')
    if (rankList && !document.querySelector('#ranking-rule')) {
      const rule = document.createElement('p')
      rule.id = 'ranking-rule'
      rule.className = 'ranking-rule'
      rule.textContent = '同分時，以猜人答案提交時間較快者排名較前。'
      rankList.insertAdjacentElement('afterend', rule)
    }
  }

  function showWinnerToast(winner) {
    let overlay = document.querySelector('#winner-phone-overlay')
    if (!overlay) {
      overlay = document.createElement('div')
      overlay.id = 'winner-phone-overlay'
      overlay.className = 'winner-phone-overlay'
      document.body.appendChild(overlay)
    }
    overlay.innerHTML = `<div class="winner-phone-card"><div>👑</div><strong>你是第一名！</strong><span>${esc(winner.playerName)}・${seconds(winner.durationMs)}</span></div>`
    overlay.classList.add('show')
    window.setTimeout(() => overlay.classList.remove('show'), 3600)
  }

  function launchConfetti(duration, count) {
    if (confettiRunning || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    confettiRunning = true

    const canvas = document.createElement('canvas')
    canvas.className = 'confetti-canvas'
    document.body.appendChild(canvas)
    const context = canvas.getContext('2d')
    const resize = () => {
      canvas.width = window.innerWidth * Math.min(window.devicePixelRatio || 1, 2)
      canvas.height = window.innerHeight * Math.min(window.devicePixelRatio || 1, 2)
      canvas.style.width = `${window.innerWidth}px`
      canvas.style.height = `${window.innerHeight}px`
      context.setTransform(canvas.width / window.innerWidth, 0, 0, canvas.height / window.innerHeight, 0, 0)
    }
    resize()
    window.addEventListener('resize', resize)

    const colors = ['#fbbf24', '#f472b6', '#a78bfa', '#38bdf8', '#34d399', '#fb7185']
    const pieces = Array.from({ length: count }, () => ({
      x: Math.random() * window.innerWidth,
      y: -20 - Math.random() * window.innerHeight * 0.45,
      width: 6 + Math.random() * 8,
      height: 8 + Math.random() * 12,
      velocityX: -2.2 + Math.random() * 4.4,
      velocityY: 2.4 + Math.random() * 4.6,
      rotation: Math.random() * Math.PI,
      rotationSpeed: -0.18 + Math.random() * 0.36,
      color: colors[Math.floor(Math.random() * colors.length)],
      wave: Math.random() * Math.PI * 2,
    }))

    const startedAt = performance.now()
    function frame(now) {
      context.clearRect(0, 0, window.innerWidth, window.innerHeight)
      for (const piece of pieces) {
        piece.wave += 0.06
        piece.x += piece.velocityX + Math.sin(piece.wave) * 0.7
        piece.y += piece.velocityY
        piece.rotation += piece.rotationSpeed
        if (piece.y > window.innerHeight + 30) {
          piece.y = -30
          piece.x = Math.random() * window.innerWidth
        }
        context.save()
        context.translate(piece.x, piece.y)
        context.rotate(piece.rotation)
        context.fillStyle = piece.color
        context.fillRect(-piece.width / 2, -piece.height / 2, piece.width, piece.height)
        context.restore()
      }
      if (now - startedAt < duration) requestAnimationFrame(frame)
      else cleanup()
    }

    function cleanup() {
      window.removeEventListener('resize', resize)
      canvas.remove()
      confettiRunning = false
    }

    requestAnimationFrame(frame)
  }

  queueMicrotask(enhancePartyUI)
})()
