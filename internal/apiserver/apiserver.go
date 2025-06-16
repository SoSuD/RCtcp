package apiserver

func Start() error {
	srv := newServer()
	srv.configureRouter()

	if err := srv.router.Run(); err != nil {
		return err
	}
	return nil
}
