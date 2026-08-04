(() => {
  const original = handleMessage
  const history = new Map()
  let speechToken = 0
  let pauseTimer = null
  let beepSecond = -1
  let audioContext = null

  handleMessage = function (message) {
    original(message)
    const type = message?.type
    const data = message?.data || {}

    if (type === 'TIMER_UPDATE' && state.status === 'answering') {
      beep(Number(data.timeLeft))
    }
    if (type === 'REVEAL_STARTED' || type === 'PROFILE_REVEALED') {
      remember(state.reveal)
      queueMicrotask(startSpeech)
    }
    if (type === 'REVEAL_COMPLETE' || type === 'GUESSING_STARTED' || type === 'GAME_FINISHED' || type === 'ROOM_RESET') {
      cancelSpeech()
    }
    queueMicrotask(enhance)
  }

  function enhance() {
    const roomInput = document.querySelector('#join-room-id')
    if (roomInput) {
      roomInput.maxLength = 4
      roomInput.inputMode = 'numeric'
      roomInput.placeholder = '例如 0527'
      if (!roomInput.dataset.onlyDigits) {
        roomInput.dataset.onlyDigits = '1'
        roomInput.addEventListener('input', () => {
          roomInput.value = roomInput.value.replace(/\D/g, '').slice(0, 4)
        })
      }
    }

    const answeringCard = state.status === 'answering' ? document.querySelector('.card.center') : null
    if (answeringCard) answeringCard.classList.add('answer-stage-card')

    if (state.status === 'revealing' && state.reveal?.profile) {
      injectRevealPanel()
    }
  }

  function remember(reveal) {
    const number = Number(reveal?.revealNumber)
    if (number > 0) history.set(number, JSON.parse(JSON.stringify(reveal)))
  }

  function injectRevealPanel() {
    document.querySelector('#reveal-auto-panel')?.remove()
    const card = document.querySelector('.card')
    if (!card) return
    card.classList.add('reveal-stage-card')

    const panel = document.createElement('div')
    panel.id = 'reveal-auto-panel'
    panel.className = 'reveal-auto-panel'
    panel.innerHTML = '<div id="reveal-auto-status" class="reveal-auto-status">正在語音朗讀…</div>'

    if (state.isHost) {
      const actions = document.createElement('div')
      actions.className = 'reveal-nav-actions'

      const previous = button('← 上一位', 'btn secondary', () => showPrevious())
      previous.disabled = Number(state.reveal.revealNumber) <= 1
      actions.appendChild(previous)
      actions.appendChild(button('重新朗讀', 'btn secondary', startSpeech))
      actions.appendChild(button(state.reveal.isLast ? '完成公布' : '下一位 →', 'btn large', nextReveal))
      panel.appendChild(actions)
    }

    card.appendChild(panel)
    const originalNext = document.querySelector('#next-reveal')
    if (originalNext) originalNext.closest('.actions')?.classList.add('hidden')
  }

  function button(text, className, click) {
    const element = document.createElement('button')
    element.type = 'button'
    element.className = className
    element.textContent = text
    element.addEventListener('click', click)
    return element
  }

  function showPrevious() {
    cancelSpeech()
    const number = Number(state.reveal?.revealNumber || 1)
    const previous = history.get(number - 1)
    if (!previous) return toast('上一位資料尚未載入')
    state.reveal = JSON.parse(JSON.stringify(previous))
    state.revealComplete = false
    render()
    queueMicrotask(() => {
      enhance()
      startSpeech()
    })
  }

  function nextReveal() {
    cancelSpeech()
    send('NEXT_REVEAL')
  }

  function startSpeech() {
    cancelSpeech()
    if (state.status !== 'revealing' || !state.reveal?.profile) return
    remember(state.reveal)

    const token = ++speechToken
    const profile = state.reveal.profile
    const items = Array.isArray(profile.answers) ? profile.answers : []

    if (!window.speechSynthesis || !window.SpeechSynthesisUtterance) {
      return pauseAfterSpeech(token)
    }

    speakText(token, profile.alias, () => speakAnswerItem(token, items, 0))
  }

  function speakAnswerItem(token, items, index) {
    if (token !== speechToken || state.status !== 'revealing') return
    if (index >= items.length) {
      pauseAfterSpeech(token)
      return
    }

    const item = items[index]
    setStatus(`正在朗讀第 ${index + 1} 題…`)
    speakText(token, `題目，${item.question}。`, () => {
      speakText(token, `答案，${item.answer}。`, () => {
        pauseTimer = window.setTimeout(() => {
          pauseTimer = null
          speakAnswerItem(token, items, index + 1)
        }, 500)
      })
    })
  }

  function speakText(token, text, onEnd) {
    if (token !== speechToken || state.status !== 'revealing') return
    const utterance = new SpeechSynthesisUtterance(text)
    utterance.lang = 'zh-TW'
    utterance.rate = 0.92
    utterance.onend = () => {
      if (token === speechToken) onEnd()
    }
    utterance.onerror = () => {
      if (token === speechToken) onEnd()
    }
    window.speechSynthesis.speak(utterance)
  }

  function pauseAfterSpeech(token) {
    if (token !== speechToken || state.status !== 'revealing') return
    let remaining = 5
    setStatus(`朗讀完成，${remaining} 秒後${state.reveal.isLast ? '完成公布' : '下一位'}`)
    pauseTimer = window.setInterval(() => {
      if (token !== speechToken) return cancelSpeech()
      remaining -= 1
      if (remaining <= 0) {
        window.clearInterval(pauseTimer)
        pauseTimer = null
        if (state.isHost) nextReveal()
        return
      }
      setStatus(`朗讀完成，${remaining} 秒後${state.reveal.isLast ? '完成公布' : '下一位'}`)
    }, 1000)
  }

  function setStatus(text) {
    const status = document.querySelector('#reveal-auto-status')
    if (status) status.textContent = text
  }

  function cancelSpeech() {
    speechToken += 1
    if (pauseTimer) {
      window.clearInterval(pauseTimer)
      window.clearTimeout(pauseTimer)
    }
    pauseTimer = null
    if (window.speechSynthesis) window.speechSynthesis.cancel()
  }

  function beep(second) {
    if (second < 1 || second > 10 || second === beepSecond) return
    beepSecond = second
    try {
      audioContext ||= new (window.AudioContext || window.webkitAudioContext)()
      const oscillator = audioContext.createOscillator()
      const gain = audioContext.createGain()
      oscillator.frequency.value = second <= 3 ? 880 : 620
      gain.gain.setValueAtTime(0.15, audioContext.currentTime)
      gain.gain.exponentialRampToValueAtTime(0.001, audioContext.currentTime + 0.14)
      oscillator.connect(gain)
      gain.connect(audioContext.destination)
      oscillator.start()
      oscillator.stop(audioContext.currentTime + 0.15)
    } catch (_) {}
  }

  const observer = new MutationObserver(() => queueMicrotask(enhance))
  observer.observe(document.querySelector('#app'), { childList: true, subtree: true })
  queueMicrotask(enhance)
})()
