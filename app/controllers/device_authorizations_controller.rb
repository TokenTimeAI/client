class DeviceAuthorizationsController < ApplicationController
  before_action :set_device_authorization

  def show
    @device_authorization.expire_if_needed!
  end

  def approve
    @device_authorization.expire_if_needed!

    if @device_authorization.expired?
      redirect_to device_authorization_path(@device_authorization.user_code),
                  alert: "This device authorization has expired."
      return
    end

    unless @device_authorization.pending?
      redirect_to device_authorization_path(@device_authorization.user_code),
                  alert: "This device authorization has already been processed."
      return
    end

    DeviceAuthorization.transaction do
      api_key = current_user.api_keys.create!(name: api_key_name(@device_authorization))
      @device_authorization.approve!(user: current_user, api_key: api_key)
    end

    redirect_to device_authorization_path(@device_authorization.user_code),
                notice: "Local heartbeat daemon approved."
  end

  private

  def set_device_authorization
    @device_authorization = DeviceAuthorization.find_by!(user_code: params[:user_code])
  end

  def api_key_name(device_authorization)
    suffix = device_authorization.machine_name.presence || "Unknown machine"
    "Local daemon - #{suffix}"
  end
end
