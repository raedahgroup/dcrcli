import { Controller } from 'stimulus'
import axios from 'axios'
import { showErrorMessage, showSuccessMessage } from '../utils'

export default class extends Controller {
  static get targets () {
    return [
      'oldPassword', 'oldPasswordError', 'newPassword', 'newPasswordError', 'confirmPassword', 'confirmPasswordError',
      'spendUnconfirmedFunds'
    ]
  }

  changePassword (e) {
    e.preventDefault()
    if (!this.validateChangePasswordFields()) {
      return
    }
    let submitBtn = e.currentTarget
    submitBtn.textContent = 'Busy...'
    submitBtn.setAttribute('disabled', true)

    let _this = this

    let postData = $('#change-password-form').serialize()
    axios.post('/change-password', postData).then((response) => {
      let result = response.data
      if (result.error !== undefined) {
        showErrorMessage(result.error)
      } else {
        _this.oldPasswordTarget.value = ''
        _this.newPasswordTarget.value = ''
        _this.confirmPasswordTarget.value = ''

        showSuccessMessage('Password changed')
      }
    }).catch(() => {
      showErrorMessage('A server error occurred')
    }).then(() => {
      submitBtn.innerHTML = 'Change Password'
      submitBtn.removeAttribute('disabled')
      $('#change-password-modal').modal('hide')
    })
  }

  validateChangePasswordFields () {
    this.clearChangePasswordValidationErrors()
    let isClean = true
    if (this.oldPasswordTarget.value === '') {
      this.oldPasswordErrorTarget.textContent = 'Old password is required'
      isClean = false
    }
    if (this.newPasswordTarget.value === '') {
      this.newPasswordErrorTarget.textContent = 'New password is required'
      isClean = false
    }
    if (this.confirmPasswordTarget.value === '') {
      this.confirmPasswordErrorTarget.textContent = 'Confirm password is required'
      isClean = false
    }
    if (this.confirmPasswordTarget.value !== this.newPasswordTarget.value) {
      if (this.confirmPasswordErrorTarget.textContent === '') {
        this.confirmPasswordErrorTarget.textContent = 'Confirm password doesn\'t match'
      }
      isClean = false
    }
    return isClean
  }

  clearChangePasswordValidationErrors () {
    this.oldPasswordErrorTarget.textContent = ''
    this.newPasswordErrorTarget.textContent = ''
    this.confirmPasswordErrorTarget.textContent = ''
  }

  updateSpendUnconfirmed () {
    const _this = this
    const postData = `spendUnconfirmed=${this.spendUnconfirmedFundsTarget.checked}`
    axios.put('/settings', postData).then((response) => {
      let result = response.data
      if (result.success) {
        showSuccessMessage('Changes saved successfully')
      } else {
        showErrorMessage(result.error ? result.error : 'Something went wrong, please try again later')
        _this.spendUnconfirmedFundsTarget.checked = !_this.spendUnconfirmedFundsTarget.checked
      }
    }).catch(() => {
      _this.spendUnconfirmedFundsTarget.checked = !_this.spendUnconfirmedFundsTarget.checked
      showErrorMessage('A server error occurred')
    })
  }
}