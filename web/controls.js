(() => {
  const originalHandleMessage = handleMessage

  handleMessage = function (message) {
    if (message?.type === 'ROOM_RESET') {
      const data = message.data || {}
      state.status = 'waiting'
      state.settings = data.settings || state.settings
      state.players = data.players || state.players
      state.question = null
      state.questionIndex = 0
      state.questionNumber = 0
      state.totalQuestions = 0
      state.timeLeft = 0
      state.submittedCount = 0
      state.hasSubmitted = false
      state.myAnswer = ''
      state.reveal = null
      state.revealComplete = false
      state.aliases = []
      state.guessPlayers = []
      state.ownAlias = ''
      state.ownPlayerId = ''
      state.guesses = {}
      state.hasGuessed = false
      state.results = []
      state.identities = []
      toast('本局已結束，返回等待房間')
      render()
      return
    }
    originalHandleMessage(message)
    queueMicrotask(injectHostControls)
  }

  function confirmAndSend(message, type) {
    if (!window.confirm(message)) return
    send(type)
  }

  function injectHostControls() {
    if (document.querySelector('#host-live-controls')) return
    if (!state.isHost || !state.roomId || state.status === 'home' || state.status === 'waiting' || state.status === 'revealing') return

    const card = document.querySelector('.card')
    if (!card) return

    const controls = document.createElement('div')
    controls.id = 'host-live-controls'
    controls.className = 'host-live-controls'

    if (state.status === 'answering') {
      const replace = document.createElement('button')
      replace.className = 'btn secondary'
      replace.textContent = '換掉這題'
      replace.addEventListener('click', () => confirmAndSend('確定換掉目前題目？本題所有已作答內容會清除，題號不變並重新完整倒數。', 'SKIP_QUESTION'))
      controls.appendChild(replace)

      const forceReveal = document.createElement('button')
      forceReveal.className = 'btn secondary'
      forceReveal.textContent = '直接進入公布'
      forceReveal.addEventListener('click', () => confirmAndSend('確定跳過剩餘作答題目，直接進入匿名同學答案公布環節？', 'FORCE_START_GUESSING'))
      controls.appendChild(forceReveal)
    }

    const reset = document.createElement('button')
    reset.className = 'btn danger'
    reset.textContent = '結束本局並返回房間'
    reset.addEventListener('click', () => confirmAndSend('確定結束本局？所有答案、猜測與分數都會清除，但玩家會保留在房間內。', 'RESET_TO_LOBBY'))
    controls.appendChild(reset)

    card.appendChild(controls)
  }

  const observer = new MutationObserver(() => queueMicrotask(injectHostControls))
  observer.observe(document.querySelector('#app'), { childList: true, subtree: true })
  queueMicrotask(injectHostControls)
})()
