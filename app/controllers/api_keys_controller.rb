class ApiKeysController < ApplicationController
  def index
    @api_keys = current_user.api_keys.order(created_at: :desc)
  end

  def create
    @api_key = current_user.api_keys.new(api_key_params)

    if @api_key.save
      # Show the key once immediately after creation
      flash[:created_key] = @api_key.key
      redirect_to api_keys_path, notice: "API key created successfully."
    else
      @api_keys = current_user.api_keys.order(created_at: :desc)
      render :index, status: :unprocessable_entity
    end
  end

  def destroy
    @api_key = current_user.api_keys.find(params[:id])
    @api_key.update!(active: false)
    redirect_to api_keys_path, notice: "API key revoked."
  end

  private

  def api_key_params
    params.require(:api_key).permit(:name)
  end
end
