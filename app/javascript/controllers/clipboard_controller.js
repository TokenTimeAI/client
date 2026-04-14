import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static values = { text: String }

  copy() {
    navigator.clipboard.writeText(this.textValue).then(() => {
      const originalText = this.element.textContent
      this.element.textContent = "Copied!"
      this.element.classList.add("text-green-400")
      setTimeout(() => {
        this.element.textContent = originalText
        this.element.classList.remove("text-green-400")
      }, 2000)
    })
  }
}
