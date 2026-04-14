class Users::OmniauthCallbacksController < Devise::OmniauthCallbacksController
  def github
    handle_oauth("GitHub")
  end

  def google_oauth2
    handle_oauth("Google")
  end

  def failure
    redirect_to new_user_session_path, alert: failure_message
  end

  private

  def handle_oauth(kind)
    @user = User.from_omniauth(request.env["omniauth.auth"])

    if @user.persisted?
      flash[:notice] = I18n.t("devise.omniauth_callbacks.success", kind: kind)
      sign_in_and_redirect @user, event: :authentication
    else
      redirect_to new_user_session_path, alert: @user.errors.full_messages.to_sentence.presence || "#{kind} sign-in failed"
    end
  end
end
