module Api
  module V1
    class ProjectsController < BaseController
      # GET /api/v1/projects
      def index
        projects = current_user.projects.order(:name)
        render json: {
          data: projects.map { |p| project_json(p) }
        }
      end

      # GET /api/v1/projects/:id
      def show
        project = current_user.projects.find(params[:id])
        render json: { data: project_json(project) }
      rescue ActiveRecord::RecordNotFound
        render json: { error: "Not found" }, status: :not_found
      end

      private

      def project_json(project)
        {
          id: project.id,
          name: project.name,
          color: project.color,
          description: project.description,
          created_at: project.created_at
        }
      end
    end
  end
end
