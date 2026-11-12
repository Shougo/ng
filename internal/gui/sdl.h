#ifndef NG_SDL_H
#define NG_SDL_H

#include <SDL3/SDL.h>

// Flattened view of the SDL_Event union fields ng uses.
typedef struct {
	Uint32 type;
	SDL_Keycode key;
	SDL_Keymod mod;
	int repeat;
	char text[64];
	float x, y;
	int button;
	Uint32 state;
} ng_event;

void ng_flatten(const SDL_Event *e, ng_event *o);
char *ng_drop_data(const SDL_Event *e);
bool ng_init(void);
void ng_wake(void);
Uint32 ng_get_wake_type(void);
int ng_wait(SDL_Event *e);
int ng_poll(SDL_Event *e);
SDL_Window *ng_create_window(const char *title, int w, int h);
void ng_fill(SDL_Renderer *r, int x, int y, int w, int h, Uint32 rgb);
SDL_Texture *ng_create_texture(SDL_Renderer *r, int access, int w, int h);
bool ng_update(SDL_Texture *t, int x, int y, int w, int h, const void *pix, int pitch);
bool ng_blit(SDL_Renderer *r, SDL_Texture *t, int w, int h);
float ng_pixel_density(SDL_Window *w);

#endif
