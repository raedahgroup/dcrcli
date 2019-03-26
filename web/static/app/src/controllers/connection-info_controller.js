import { Controller } from 'stimulus'
import ws from '../services/messagesocket_service'

export default class extends Controller {
  static get targets () {
    return [
      'container',
      'totalBalance',
      'peersConnected',
      'latestBlock',
      'networkType'
    ]
  }

  connect () {
    let _this = this

    ws.registerEvtHandler('updateConnInfo', function (data) {
      _this.peersConnectedTarget.textContent = data.peersConnected
      _this.totalBalanceTarget.textContent = data.totalBalance
      _this.latestBlockTarget.textContent = data.latestBlock
      _this.networkTypeTarget.textContent = data.networkType

      _this.show(_this.containerTarget)
    })

    ws.registerEvtHandler('updateBalance', function (data) {
      _this.totalBalanceTarget.textContent = data
    })
  }

  hide (el) {
    el.classList.add('d-none')
  }

  show (el) {
    el.classList.remove('d-none')
  }
}
